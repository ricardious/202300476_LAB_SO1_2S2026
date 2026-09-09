// SPDX-License-Identifier: GPL-2.0
/*
 * continfo.c — Módulo de kernel "Sonda de contenedores" (Proyecto 2, SO1)
 * Carnet: 202300476 · Sistemas Operativos 1 · 2do Semestre 2026
 *
 * Expone un documento JSON consistente en
 *   /proc/continfo_pr2_so1_202300476
 * con:
 *   - memoria total / libre / usada (en KB),
 *   - por cada proceso (generales y de contenedores):
 *       pid, name, cmdline, container_id, vsz_kb, rss_kb,
 *       memory_percent, cpu_percent.
 *
 * Diseño (ver PLAN_PROYECTO2.md, sección 3.1):
 *   - Salida mediante seq_file (single_open) para una lectura consistente.
 *   - Se toman referencias (get_task_struct / get_task_mm) bajo RCU y se
 *     formatea la salida FUERA de cualquier bloqueo de tareas.
 *   - Identificación de contenedores a partir de la ruta de cgroup
 *     (token hexadecimal de 64 chars estilo Docker); `cmdline` se conserva
 *     como alternativa verificable cuando no hay cgroup.
 *   - CPU: promedio sobre la vida del proceso, misma convención que `ps`
 *     (utime+stime / tiempo transcurrido). Documentado en el manual.
 *
 * Compatibilidad: kernel >= 5.6 (struct proc_ops) con guarda para
 * versiones anteriores. Funciona con cgroup v1 y v2.
 */

#include <linux/init.h>
#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/version.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/fs.h>
#include <linux/sysinfo.h>
#include <linux/sched.h>
#include <linux/sched/signal.h>
#include <linux/sched/mm.h>
#include <linux/mm.h>
#include <linux/mmap_lock.h>
#include <linux/ktime.h>
#include <linux/timekeeping.h>
#include <linux/slab.h>
#include <linux/ctype.h>
#include <linux/string.h>

#define MODNAME "continfo"
#define DRIVER_AUTHOR "Carnet 202300476"
#define DRIVER_DESC "Telemetria de memoria y contenedores via procfs"

/* Contrato fijo del proyecto. */
#define PROC_NAME "continfo_pr2_so1_202300476"

/* Cota de procesos por instantánea (protección contra memoria ilimitada). */
#define MAX_TASKS_SNAPSHOT 65536

/* Tamaño máximo de cmdline reportada por proceso. */
#define CMDLINE_MAX 512

/* Tamaño del buffer de cgroup por proceso. */
#define CGROUP_BUF_MAX 4096

/*
 * Tiempo de CPU acumulado en nanosegundos.
 * Desde Linux ~4.8 la cputime de task_struct (utime/stime) siempre se
 * almacena en nanosegundos, independientemente de si la contabilidad se
 * basa en ticks (TICK_CPU_ACCOUNTING) o en contabilidad virtual
 * (VIRT_CPU_ACCOUNTING_NATIVE/GEN); la diferencia solo afecta a cómo se
 * obtiene el delta, no a la unidad. Se verifica en las pruebas contra ps.
 */
#define CPU_TIME_NS(p) ((unsigned long long)((p)->utime + (p)->stime))

static struct proc_dir_entry *continfo_proc_entry;

/* ---------------------------------------------------------------------
 * Utilidades JSON
 * ------------------------------------------------------------------- */

/*
 * Emite una cadena JSON válida, escapando caracteres de control,
 * comillas y backslashes. `len` puede ser menor que el tamaño del buffer
 * (las cadenas pueden no estar NUL-terminadas, p. ej. cmdline).
 */
static void emit_json_string(struct seq_file *m, const char *src, size_t len)
{
	static const char hex[] = "0123456789abcdef";
	size_t i;
	unsigned char c;

	seq_putc(m, '"');
	for (i = 0; i < len; i++) {
		c = (unsigned char)src[i];
		switch (c) {
		case '"':
			seq_puts(m, "\\\"");
			break;
		case '\\':
			seq_puts(m, "\\\\");
			break;
		case '\n':
			seq_puts(m, "\\n");
			break;
		case '\r':
			seq_puts(m, "\\r");
			break;
		case '\t':
			seq_puts(m, "\\t");
			break;
		default:
			if (c < 0x20) {
				seq_puts(m, "\\u00");
				seq_putc(m, hex[(c >> 4) & 0xf]);
				seq_putc(m, hex[c & 0xf]);
			} else {
				seq_putc(m, c);
			}
		}
	}
	seq_putc(m, '"');
}

/* ---------------------------------------------------------------------
 * Datos por proceso
 * ------------------------------------------------------------------- */

/* Buffers reutilizados entre procesos. */
struct proc_scratch {
	char cmdline[CMDLINE_MAX];
	char cgroup[CGROUP_BUF_MAX];
};

/*
 * Lee la línea de comando con la misma semántica que /proc/<pid>/cmdline:
 * los NUL interiores que separan argv[] se convierten en espacios.
 * Usa únicamente funciones exportadas (get_task_mm, access_process_vm).
 */
static void read_cmdline(struct task_struct *task, char *buf, size_t buflen)
{
	struct mm_struct *mm;
	unsigned long arg_start, arg_end;
	size_t len;
	ssize_t res;
	int i;

	if (!buf || buflen == 0)
		return;

	buf[0] = '\0';
	mm = get_task_mm(task);
	if (!mm)
		return;
	if (!mm->arg_end)
		goto out;

	mmap_read_lock(mm);
	arg_start = mm->arg_start;
	arg_end = mm->arg_end;
	mmap_read_unlock(mm);

	if (arg_end <= arg_start)
		goto out;

	len = arg_end - arg_start;
	if (len > buflen - 1)
		len = buflen - 1;

	res = access_process_vm(task, arg_start, buf, len, FOLL_FORCE);
	if (res <= 0)
		goto out;

	/* Los argv[] están separados por NUL: se muestran como espacios. */
	for (i = 0; i < res; i++) {
		if (buf[i] == '\0')
			buf[i] = ' ';
	}
	buf[res] = '\0';

out:
	mmput(mm);
}

/*
 * Lee /proc/<pid>/cgroup y extrae un posible container_id.
 * Convenciones soportadas:
 *   - cgroup v2 -> 0::/docker/<64hex>            o 0::/system.slice/docker-<64hex>.scope
 *   - cgroup v1 -> 1:name=systemd:/docker/<64hex>
 * Docker usa identificadores de 64 chars hexadecimales; se acepta además
 * cualquier token >= 32 chars como identificador (podman/buildkit).
 * Devuelve 1 si se halló un identificador, 0 en caso contrario.
 */
static int read_container_id(struct task_struct *task, char *out, size_t outlen)
{
	char path[32];
	struct file *f;
	loff_t pos = 0;
	ssize_t n;
	int best_start = -1, best_len = 0;
	int cur_start = -1, cur_len = 0;
	int i;
	int found = 0;

	if (!out || outlen == 0)
		return 0;

	out[0] = '\0';
	snprintf(path, sizeof(path), "/proc/%d/cgroup", task_pid_nr(task));

	f = filp_open(path, O_RDONLY, 0);
	if (IS_ERR(f))
		return 0;

	n = kernel_read(f, out, outlen - 1, &pos);
	if (n <= 0) {
		filp_close(f, NULL);
		return 0;
	}
	out[n] = '\0';

	/* Barrido para localizar el token hexadecimal más largo. */
	for (i = 0; i < n; i++) {
		unsigned char c = (unsigned char)out[i];

		if (isxdigit(c)) {
			if (cur_start < 0)
				cur_start = i;
			cur_len++;
		} else {
			if (cur_len > best_len) {
				best_len = cur_len;
				best_start = cur_start;
			}
			cur_start = -1;
			cur_len = 0;
		}
	}
	if (cur_len > best_len) {
		best_len = cur_len;
		best_start = cur_start;
	}

	/* Solo consideramos identificadores razonables (>= 32 hex). */
	if (best_start >= 0 && best_len >= 32) {
		size_t copy = min_t(size_t, best_len, outlen - 1);

		memmove(out, &out[best_start], copy);
		out[copy] = '\0';
		found = 1;
	}

	filp_close(f, NULL);
	return found;
}

/* ---------------------------------------------------------------------
 * snapshot de procesos
 * ------------------------------------------------------------------- */

/*
 * Toma una referencia (get_task_struct) de cada tarea del sistema.
 * Se ejecuta bajo RCU solamente mientras se copian los punteros; el
 * trabajo pesado (mm, cmdline, cgroup) se hace después, fuera de RCU.
 * Devuelve el número de referencias tomadas (0 a cap).
 */
static unsigned int snapshot_tasks(struct task_struct **tasks, unsigned int cap)
{
	struct task_struct *p;
	unsigned int n = 0;

	rcu_read_lock();
	for_each_process(p) {
		if (n >= cap)
			break;
		get_task_struct(p);
		tasks[n++] = p;
	}
	rcu_read_unlock();
	return n;
}

static void release_tasks(struct task_struct **tasks, unsigned int n)
{
	unsigned int i;

	for (i = 0; i < n; i++)
		put_task_struct(tasks[i]);
}

/* ---------------------------------------------------------------------
 * show() — construye el documento JSON completo
 * ------------------------------------------------------------------- */

static int continfo_show(struct seq_file *m, void *v)
{
	struct task_struct **tasks;
	struct proc_scratch *scratch;
	unsigned int n, i;
	struct sysinfo si;
	unsigned long long total_kb, free_kb, used_kb;
	u64 now_ns;
	int first = 1;

	/* Memoria: totalram_pages()/nr_free_pages() = MemTotal/MemFree. */
	si_meminfo(&si);
	total_kb = (unsigned long long)si.totalram << (PAGE_SHIFT - 10);
	free_kb = (unsigned long long)si.freeram << (PAGE_SHIFT - 10);
	used_kb = total_kb - free_kb;

	now_ns = ktime_get_real_ns();

	tasks = kvmalloc_array(MAX_TASKS_SNAPSHOT, sizeof(*tasks), GFP_KERNEL);
	if (!tasks) {
		seq_puts(m, "{\"schema_version\":1,\"error\":\"ENOMEM\"}\n");
		return 0;
	}
	scratch = kzalloc(sizeof(*scratch), GFP_KERNEL);
	if (!scratch) {
		kvfree(tasks);
		seq_puts(m, "{\"schema_version\":1,\"error\":\"ENOMEM\"}\n");
		return 0;
	}

	n = snapshot_tasks(tasks, MAX_TASKS_SNAPSHOT);

	seq_puts(m, "{\n");
	seq_printf(m, "  \"schema_version\": %d,\n", 1);
	seq_printf(m, "  \"timestamp_unix_ns\": %llu,\n", now_ns);
	seq_printf(m, "  \"memory\": {\n");
	seq_printf(m, "    \"total_kb\": %llu,\n", total_kb);
	seq_printf(m, "    \"free_kb\": %llu,\n", free_kb);
	seq_printf(m, "    \"used_kb\": %llu\n", used_kb);
	seq_puts(m, "  },\n");
	seq_puts(m, "  \"processes\": [\n");

	for (i = 0; i < n; i++) {
		struct task_struct *p = tasks[i];
		struct mm_struct *mm;
		unsigned long long vsz_kb = 0, rss_kb = 0;
		unsigned long long mem_pct_int = 0, mem_pct_frac = 0;
		unsigned long long cpu_pct_int = 0, cpu_pct_frac = 0;
		unsigned long long cpu_ns, elapsed_ns;
		size_t clen = 0;
		int cid_found = 0;
		pid_t pid;

		/* ---------- datos que requieren mm (vsz, rss) ---------- */
		mm = get_task_mm(p);
		if (mm) {
			vsz_kb = (unsigned long long)mm->total_vm << (PAGE_SHIFT - 10);
			rss_kb = (unsigned long long)get_mm_rss(mm) << (PAGE_SHIFT - 10);
			mmput(mm);
		}

		/* ---------- cmdline (alternativa verificable) ---------- */
		read_cmdline(p, scratch->cmdline, sizeof(scratch->cmdline));
		clen = strnlen(scratch->cmdline, sizeof(scratch->cmdline));

		/* ---------- contenedor (cgroup) ---------- */
		cid_found = read_container_id(p, scratch->cgroup,
					      sizeof(scratch->cgroup));

		/* ---------- porcentaje de memoria ---------- */
		if (total_kb > 0) {
			unsigned long long pct = (rss_kb * 10000ULL) / total_kb;

			mem_pct_int = pct / 100;
			mem_pct_frac = pct % 100;
		}

		/*
		 * CPU: promedio sobre la vida del proceso, igual que `ps`.
		 *   cpu% = (utime+stime) / elapsed * 100
		 */
		cpu_ns = CPU_TIME_NS(p);
		elapsed_ns = (unsigned long long)(ktime_get_ns() - p->start_time);
		if (elapsed_ns > 0) {
			unsigned long long pct = (cpu_ns * 10000ULL) / elapsed_ns;

			cpu_pct_int = pct / 100;
			cpu_pct_frac = pct % 100;
		}

		pid = task_pid_nr(p);

		seq_printf(m, "    %s{\n", first ? "" : ",");
		seq_printf(m, "      \"pid\": %d,\n", pid);
		seq_printf(m, "      \"name\": ");
		emit_json_string(m, p->comm, strnlen(p->comm, TASK_COMM_LEN));
		seq_puts(m, ",\n");
		seq_printf(m, "      \"cmdline\": ");
		emit_json_string(m, scratch->cmdline, clen);
		seq_puts(m, ",\n");
		seq_printf(m, "      \"container_id\": ");
		if (cid_found)
			emit_json_string(m, scratch->cgroup,
					 strnlen(scratch->cgroup,
						 sizeof(scratch->cgroup)));
		else
			seq_puts(m, "\"\"");
		seq_puts(m, ",\n");
		seq_printf(m, "      \"vsz_kb\": %llu,\n", vsz_kb);
		seq_printf(m, "      \"rss_kb\": %llu,\n", rss_kb);
		seq_printf(m, "      \"memory_percent\": %llu.%02llu,\n",
			   mem_pct_int, mem_pct_frac);
		seq_printf(m, "      \"cpu_percent\": %llu.%02llu\n",
			   cpu_pct_int, cpu_pct_frac);
		seq_puts(m, "    }");
		first = 0;
	}

	release_tasks(tasks, n);
	kvfree(tasks);
	kfree(scratch);

	seq_puts(m, "\n  ]\n");
	seq_puts(m, "}\n");
	return 0;
}

/* ---------------------------------------------------------------------
 * interfaz /proc
 * ------------------------------------------------------------------- */

static int continfo_open(struct inode *inode, struct file *file)
{
	return single_open(file, continfo_show, NULL);
}

#if LINUX_VERSION_CODE >= KERNEL_VERSION(5, 6, 0)
static const struct proc_ops continfo_fops = {
	.proc_open = continfo_open,
	.proc_read = seq_read,
	.proc_lseek = seq_lseek,
	.proc_release = single_release,
};
#else
static const struct file_operations continfo_fops = {
	.owner = THIS_MODULE,
	.open = continfo_open,
	.read = seq_read,
	.llseek = seq_lseek,
	.release = single_release,
};
#endif

/* ---------------------------------------------------------------------
 * init / exit
 * ------------------------------------------------------------------- */

static int __init continfo_init(void)
{
	continfo_proc_entry = proc_create(PROC_NAME, 0444, NULL, &continfo_fops);
	if (!continfo_proc_entry) {
		pr_err(MODNAME ": no se pudo crear /proc/%s\n", PROC_NAME);
		return -ENOMEM;
	}
	pr_info(MODNAME ": modulo cargado, /proc/%s disponible\n", PROC_NAME);
	return 0;
}

static void __exit continfo_exit(void)
{
	proc_remove(continfo_proc_entry);
	continfo_proc_entry = NULL;
	pr_info(MODNAME ": modulo descargado, /proc/%s eliminado\n", PROC_NAME);
}

module_init(continfo_init);
module_exit(continfo_exit);

MODULE_LICENSE("GPL");
MODULE_AUTHOR(DRIVER_AUTHOR);
MODULE_DESCRIPTION(DRIVER_DESC);
MODULE_VERSION("1.0");
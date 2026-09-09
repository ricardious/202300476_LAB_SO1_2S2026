CAPTURA: README (portada)
Comando: bash -c head -n 20 /home/ricardious/Proyectos/202300476_LAB_SO1_2S2026/README.md
Fecha: vie 21 ago 2026 00:41:45 CST
----
# 202300476_LAB_SO1_2S2026

**Proyecto 1 · Sistemas Operativos 1 · 2do Semestre 2026**
Universidad de San Carlos de Guatemala — Facultad de Ingeniería

> **Carnet:** 202300476

---

## Objetivo

Diseñar e implementar un entorno virtualizado (KVM) que integre máquinas virtuales y
contenedores para simular un entorno real de desarrollo: **3 APIs REST en Go**
contenedorizadas, un **registro privado de imágenes (Zot)** y **comunicación cruzada**
entre servicios mediante REST/HTTP con JSON.

## Arquitectura

3 máquinas virtuales sobre KVM, cada una con un runtime de contenedores distinto:


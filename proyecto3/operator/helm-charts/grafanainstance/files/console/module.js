/* Local SystemJS panel. React and PanelPlugin are supplied by Grafana. */
System.register(["react", "@grafana/data"], function (exports) {
  "use strict";
  let React, PanelPlugin;
  const countries = [
    { code: "USA", name: "Estados Unidos", lon: -100, lat: 39 },
    { code: "GTM", name: "Guatemala", lon: -90.2, lat: 15.7 },
    { code: "ESP", name: "España", lon: -3.7, lat: 40.4 },
    { code: "RUS", name: "Rusia", lon: 100, lat: 60 },
    { code: "CHN", name: "China", lon: 104, lat: 35 },
  ];
  const number = new Intl.NumberFormat("es-GT", { maximumFractionDigits: 0 });
  const fmt = n => Number.isFinite(n) ? number.format(n) : "—";
  function records(data, ref) {
    if (data.error || data.state === "Error") return [];
    return (data.series || []).filter(s => s.refId === ref).flatMap(s =>
      s.fields.filter(f => f.type === "number").map(f => {
        // An absent final sample must not silently resurrect a previous value.
        const n = f.values[f.values.length - 1];
        return { value: typeof n === "number" && Number.isFinite(n) ? n : null, labels: f.labels || {} };
      })
    );
  }
  function value(data, ref, country) {
    const rows = records(data, ref);
    return (country ? rows.find(r => r.labels.country === country) : rows[0])?.value ?? null;
  }
  function ranking(data, ref) {
    return records(data, ref).filter(r => r.value !== null)
      .sort((a, b) => b.value - a.value || (a.labels.country || "").localeCompare(b.labels.country || ""));
  }
  // Grafana 13 kiosk keeps a viewport-sized shell. Contain dashboard scrolling
  // in its page wrapper so the sibling footer never overlays the panel grid.
  // Restrict this shell adjustment to dashboards containing this console.
  const css = `
body:has(.so1-console) div:has(+ [data-testid="public-dashboard-footer"]){overflow:auto;min-height:0}
body:has(.so1-console) [data-testid="public-dashboard-footer"]{position:static;flex-shrink:0}

.so1-console{--ink:#dcece9;--muted:#9db6bd;--air:#81baf2;--sea:#77d7d2;--focus:#f2bf77;color:var(--ink);background:#0b232e;border:1px solid #35505a;border-radius:7px;font-family:"Segoe UI",Arial,sans-serif;height:100%;overflow:auto;container-type:inline-size;scrollbar-color:#35505a #0b232e}
.so1-console *{box-sizing:border-box}.so1-console h1,.so1-console h2,.so1-console h3,.so1-console p{margin:0}.so1-console button{font:inherit;cursor:pointer}.so1-console button:focus-visible,.so1-console [role=button]:focus-visible{outline:2px solid var(--focus);outline-offset:3px}.so1-console .mono{font-family:"Liberation Mono",Consolas,monospace;font-variant-numeric:tabular-nums}.so1-console .muted{color:var(--muted)}
.so1-console .masthead{display:flex;justify-content:space-between;align-items:center;padding:20px 26px;border-bottom:1px solid #35505a;gap:12px}.so1-console .identity{display:flex;align-items:center;gap:16px}.so1-console .identity svg{width:46px;height:46px;flex:none}.so1-console h1{font-size:25px;letter-spacing:1px;font-weight:650}.so1-console .subtitle{color:var(--muted);font-size:12px;margin-top:4px}.so1-console .status{text-align:right;font-size:12px;line-height:1.8}.so1-console .status strong{color:var(--sea);font-weight:500}.so1-console .warning{color:#ffd39c;padding:10px 24px;border-bottom:1px solid #8c6840;background:#3b3024;font-size:13px}
.so1-console .countries{display:flex;border-bottom:1px solid #35505a;background:#102b36}.so1-console .country{display:flex;align-items:center;gap:10px;flex:1;padding:13px 18px;background:transparent;color:var(--muted);border:0;border-bottom:2px solid transparent;text-align:left;white-space:nowrap}.so1-console .country:hover{background:#193b47;color:#fff}.so1-console .country[aria-pressed=true]{color:var(--focus);border-bottom-color:var(--focus);background:#20353a}.so1-console .country span{font-size:12px}.so1-console .country b{font-size:13px}
.so1-console .theater{display:grid;grid-template-columns:minmax(0,1fr) 260px;border-bottom:1px solid #35505a}.so1-console .map{position:relative;min-width:0;padding:18px 20px 10px;background:#091f2a}.so1-console .map-title{display:flex;align-items:center;justify-content:space-between;font-size:12px;color:var(--muted)}.so1-console .map-title b{color:var(--ink);font-weight:500}.so1-console .map svg{display:block;width:100%;height:300px}.so1-console .map-note{font-size:11px;color:var(--muted);line-height:1.5}.so1-console .marker{cursor:pointer}.so1-console .marker text{font:12px "Liberation Mono",monospace;fill:#cee2e3;paint-order:stroke;stroke:#091f2a;stroke-width:4px;stroke-linejoin:round}.so1-console .detail{padding:24px 22px;border-left:1px solid #35505a;background:#102b36}.so1-console .detail .code{color:var(--focus);font-size:12px;letter-spacing:3px}.so1-console .detail h2{font-size:30px;margin-top:7px;font-weight:500}.so1-console .detail .assigned{font-size:12px;color:var(--muted);margin-top:8px;min-height:18px}.so1-console .readings{display:flex;gap:26px;margin:23px 0}.so1-console .reading strong{display:block;font-size:36px;font-weight:400;line-height:1.3}.so1-console .reading span{font-size:12px}.so1-console .air{color:var(--air)}.so1-console .sea{color:var(--sea)}.so1-console .report-count{border-top:1px solid #35505a;padding-top:17px;display:flex;justify-content:space-between;align-items:center;gap:8px;font-size:12px;color:var(--muted)}.so1-console .report-count b{color:var(--ink);font-size:23px;font-weight:400}.so1-console .detail-note{font-size:11px;color:var(--muted);margin-top:15px;line-height:1.6}
.so1-console .instruments{display:grid;grid-template-columns:1fr 1fr;border-bottom:1px solid #35505a}.so1-console .instrument{padding:18px 24px;display:grid;grid-template-columns:100px 1fr 1fr 1.4fr;gap:14px;align-items:start}.so1-console .instrument+.instrument{border-left:1px solid #35505a}.so1-console .instrument h3{font-size:14px;font-weight:500;padding-top:5px}.so1-console .instrument small{display:block;font-size:10px;color:var(--muted);line-height:1.6;margin-top:5px}.so1-console .dial span{font-size:11px;color:var(--muted)}.so1-console .dial b{display:block;font-size:26px;font-weight:400;margin-top:3px}.so1-console .ties{font-size:10px;line-height:1.5;color:var(--muted);margin-top:4px;overflow-wrap:anywhere}
.so1-console .rankings{display:grid;grid-template-columns:1fr 1fr}.so1-console .rank{padding:18px 24px}.so1-console .rank+.rank{border-left:1px solid #35505a}.so1-console .rank h3{font-size:13px;font-weight:500;margin-bottom:13px;display:flex;justify-content:space-between}.so1-console .rank h3 span{font-size:11px;color:var(--muted)}.so1-console .rank-row{width:100%;display:grid;grid-template-columns:28px 42px 1fr 72px;align-items:center;gap:8px;border:0;background:transparent;color:var(--muted);font-size:12px;padding:5px 0;text-align:left}.so1-console .rank-row:hover{background:#193b47}.so1-console .rank-row[aria-pressed=true]{color:var(--focus)}.so1-console .track{height:5px;background:#1d3c48;overflow:hidden}.so1-console .track i{height:100%;display:block;background:currentColor}.so1-console .rank-row b{text-align:right;color:var(--ink);font-weight:400}.so1-console .rank-row .position{font-size:10px}.so1-console .foot{padding:12px 24px;border-top:1px solid #35505a;display:flex;flex-wrap:wrap;gap:6px 24px;justify-content:space-between;font-size:10px;color:var(--muted)}
@container (max-width:900px){.so1-console .country span{display:none}.so1-console .theater{grid-template-columns:minmax(0,1fr) 215px}.so1-console .detail{padding:20px 15px}.so1-console .instrument{grid-template-columns:65px 1fr 1fr 1.3fr;padding:16px 12px;gap:8px}.so1-console .map svg{height:250px}}
@container (max-width:600px){.so1-console .masthead{padding:16px;align-items:flex-start}.so1-console h1{font-size:20px}.so1-console .identity{gap:10px}.so1-console .identity svg{width:32px;height:32px}.so1-console .status{font-size:10px}.so1-console .country{justify-content:center;padding:12px 4px}.so1-console .theater,.so1-console .instruments,.so1-console .rankings{grid-template-columns:1fr}.so1-console .map{padding:15px 10px}.so1-console .map svg{height:200px}.so1-console .detail{border-left:0;border-top:1px solid #35505a;padding:20px}.so1-console .detail h2{font-size:26px}.so1-console .readings{margin:15px 0}.so1-console .instrument+.instrument,.so1-console .rank+.rank{border-left:0;border-top:1px solid #35505a}.so1-console .instrument{padding:16px}.so1-console .subtitle{font-size:10px}.so1-console .rank{padding:16px}.so1-console .foot{padding:14px 16px}}
`;
  function ConsolePanel({data, width}) {
    const h = React.createElement;
    const [selected, select] = React.useState("ESP");
    const country = countries.find(c => c.code === selected);
    const error = data.error || data.state === "Error";
    const ready = (data.series || []).some(s => s.fields.some(f => f.type === "number" && f.values.length));
    const status = error ? "Consulta no disponible" : data.state === "Loading" ? "Actualizando consulta" : ready ? "Datos consultados" : "Sin datos";
    function metric(kind, label, tone) {
      const ties = records(data, kind + "_mode")[0]?.labels.ties;
      return h("section", {className:"instrument " + tone, key:kind},
        h("h3", null, label, h("small", null, "Global · histórico")),
        ...[["min", "Mínimo"], ["max", "Máximo"], ["mode", "Moda"]].map(([key, title]) => h("div", {className:"dial",key},
          h("span",null,title),h("b",{className:"mono"},fmt(value(data,kind+"_"+key))),
          key === "mode" && h("div",{className:"ties"},ties ? "Valores: " + ties : "Sin observaciones"))));
    }
    function rank(ref, title, tone) {
      const rows = ranking(data, ref);const max = rows[0]?.value || 1;
      return h("section",{className:"rank",key:ref},h("h3",{className:tone},title,h("span",null,"Unidades acumuladas")),
        rows.length ? rows.map((r,i)=>h("button",{className:"rank-row",key:r.labels.country,"aria-pressed":r.labels.country===selected,"aria-label":"Seleccionar "+r.labels.country+" en "+title,onClick:()=>select(r.labels.country)},
          h("span",{className:"position mono"},String(i+1).padStart(2,"0")),h("span",{className:"mono"},r.labels.country),
          h("span",{className:"track "+tone},h("i",{style:{width:(100*r.value/max)+"%"}})),h("b",{className:"mono"},fmt(r.value)))):
          h("p",{className:"muted"},"Sin reportes disponibles"));
    }
    const x=(country.lon+180)*2.5,y=(85-country.lat)*3;
    return h("div",{className:"so1-console",style:{width},"aria-label":"Consola de operaciones MUMN"},
      h("style",null,css),
      h("header",{className:"masthead"},h("div",{className:"identity"},
        h("svg",{viewBox:"0 0 60 60","aria-hidden":true},h("g",{fill:"none",stroke:"#77d7d2",strokeWidth:1.5},h("circle",{cx:30,cy:30,r:24}),h("circle",{cx:30,cy:30,r:13}),h("path",{d:"M30 2V58M2 30H58M30 30L47 13"}),h("circle",{cx:47,cy:13,r:3,fill:"#f2bf77",stroke:"none"}))),
        h("div",null,h("h1",null,"M.U.M.N."),h("p",{className:"subtitle"},"Consola aérea y marítima"))),
        h("div",{className:"status"},h("strong",null,status),h("div",{className:"mono"},"SO1 / 202300476"))),
      error && h("div",{className:"warning",role:"alert"},"No se pudo consultar la fuente de datos. Revisa Prometheus y actualiza el dashboard."),
      h("nav",{className:"countries","aria-label":"País observado"},countries.map(c=>h("button",{key:c.code,className:"country","aria-pressed":selected===c.code,onClick:()=>select(c.code)},h("b",{className:"mono"},c.code),h("span",null,c.name)))),
      h("div",{className:"theater"},h("section",{className:"map","aria-label":"Mapa de los cinco países"},
        h("div",{className:"map-title"},h("b",null,"Área de observación"),h("span",{className:"mono"},"5 países / ESP asignado")),
        h("svg",{viewBox:"0 0 900 435",role:"group","aria-label":"Selecciona un país en el mapa"},
          h("g",{stroke:"#25424e",strokeWidth:.7,strokeDasharray:"2 5"},...[75,150,225,300,375].map(v=>h("path",{key:"h"+v,d:`M0 ${v}H900`})),...[75,150,225,300,375,450,525,600,675,750,825].map(v=>h("path",{key:"v"+v,d:`M${v} 0V435`}))),
          h("image",{href:"public/plugins/so1-operations-panel/world.svg",width:900,height:435}),
          h("g",{stroke:"#f2bf77",fill:"none",opacity:.35,strokeWidth:.8},h("circle",{cx:x,cy:y,r:42}),h("circle",{cx:x,cy:y,r:65}),h("path",{d:`M${x-80} ${y}H${x+80}M${x} ${y-80}V${y+80}`,strokeDasharray:"3 5"})),
          countries.map(c=>h("g",{key:c.code,className:"marker",role:"button",tabIndex:0,"aria-label":"Observar "+c.name,"aria-pressed":selected===c.code,onClick:()=>select(c.code),onKeyDown:e=>{if(e.key==="Enter"||e.key===" "){e.preventDefault();select(c.code);}},transform:`translate(${(c.lon+180)*2.5},${(85-c.lat)*3})`},h("circle",{r:18,fill:"transparent"}),h("circle",{r:selected===c.code?6:4,fill:selected===c.code?"#f2bf77":"#77d7d2",stroke:"#091f2a",strokeWidth:2}),h("text",{x:12,y:4},c.code)))),
        h("p",{className:"map-note"},"Referencias geográficas por país · No representa posiciones de vehículos")),
        h("aside",{className:"detail","aria-label":"Detalle del país seleccionado"},h("div",{className:"code mono"},selected),h("h2",null,country.name),h("p",{className:"assigned"},selected==="ESP"?"País asignado al proyecto":"País observado"),
          h("div",{className:"readings"},...[ ["planes_latest","Aviones","air"],["ships_latest","Barcos","sea"] ].map(([ref,title,tone])=>h("div",{className:"reading "+tone,key:ref},h("strong",{className:"mono"},fmt(value(data,ref,selected))),h("span",null,title)))),
          h("div",{className:"report-count"},h("span",null,"Reportes acumulados"),h("b",{className:"mono"},fmt(value(data,"reports",selected)))),h("p",{className:"detail-note"},"Aviones y barcos: último estado recibido por fecha del evento. Una consulta reciente no implica un reporte nuevo."))),
      h("div",{className:"instruments"},metric("planes","Aviones","air"),metric("ships","Barcos","sea")),
      h("div",{className:"rankings"},rank("planes_rank","Top 5 · Aviones","air"),rank("ships_rank","Top 5 · Barcos","sea")),
      h("footer",{className:"foot"},h("span",null,"Agregados históricos · Los rankings suman unidades reportadas, no vehículos únicos."),h("span",{className:"mono"},"Consulta / 10 s · Mapa / Natural Earth")));
  }
  return {
    setters: [m=>{React=m;},m=>{PanelPlugin=m.PanelPlugin;}],
    execute: function () { exports("plugin", new PanelPlugin(ConsolePanel)); exports("records",records); exports("ranking",ranking); exports("value",value); }
  };
});

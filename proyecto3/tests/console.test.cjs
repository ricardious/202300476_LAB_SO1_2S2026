const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const path = require("node:path");
const api = {};
let Component;
const react = {createElement:(tag,props,...children)=>({tag,props,children}),useState:()=>["ESP",()=>{}]};
vm.runInNewContext(fs.readFileSync(path.join(__dirname,"../operator/helm-charts/grafanainstance/files/console/module.js"),"utf8"), {
  Intl, System: { register(deps, factory) {
    const module = factory((key, value)=>{api[key]=value;});
    module.setters[0](react);
    module.setters[1]({PanelPlugin:class {constructor(component){Component=component;}}});
    module.execute();
  }}
});
const frame = (refId,values,labels={})=>({refId,fields:[{type:"time",values:[1,2]},{type:"number",values,labels}]});
test("zero is a measurement; absent, null and nonfinite samples are missing",()=>{
  for(const [samples,expected] of [[[3,0],0],[[3,null],null],[[3,NaN],null],[[],null]]) {
    assert.equal(api.value({series:[frame("planes",samples)]},"planes"),expected);
  }
  assert.equal(api.value({series:[]},"planes"),null);
});
test("country values are isolated and modal ties retained",()=>{
  const data={series:[frame("reports",[8],{country:"GTM"}),frame("reports",[0],{country:"ESP"}),frame("mode",[2],{ties:"2,4"})]};
  assert.equal(api.value(data,"reports","ESP"),0);
  assert.equal(api.value(data,"reports","USA"),null);
  assert.equal(api.records(data,"mode")[0].labels.ties,"2,4");
});
test("ranking sorts descending, breaks ties by country and excludes missing values",()=>{
  const rows=[frame("rank",[5],{country:"USA"}),frame("rank",[10],{country:"ESP"}),frame("rank",[5],{country:"GTM"}),frame("rank",[null],{country:"RUS"})];
  assert.equal(api.ranking({series:rows},"rank").map(r=>r.labels.country).join(","),"ESP,GTM,USA");
});
test("query failure never presents retained measurements as current",()=>{
  const series=[frame("planes",[42])];
  assert.equal(api.value({series,state:"Error"},"planes"),null);
  assert.equal(api.value({series,error:{message:"unavailable"}},"planes"),null);
  assert.ok(JSON.stringify(Component({data:{series,state:"Error"},width:1200})).includes("No se pudo consultar"));
});
test("empty data renders a meaningful state without throwing",()=>{
  const tree=JSON.stringify(Component({data:{series:[],state:"Done"},width:390}));
  assert.ok(tree.includes("Sin datos"));
  assert.ok(tree.includes("Sin reportes disponibles"));
});

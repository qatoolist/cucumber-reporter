import json,sys,os
D=sys.argv[1]
def st(name,status,kw="Given ",line=4,after=None,before=None):
    s={"keyword":kw,"name":name,"line":line,"match":{"location":"s.rb:1"},"result":{"status":status,"duration":1}}
    if status=="failed": s["result"]["error_message"]="E-"+name
    if after: s["after"]=after
    if before: s["before"]=before
    return s
def hook(status): 
    h={"match":{"location":"h.rb:1"},"result":{"status":status,"duration":1}}
    if status=="failed": h["result"]["error_message"]="HOOK"
    return h
def sc(steps,before=None,after=None,typ="scenario",name="s",line=3,kw="Scenario"):
    e={"keyword":kw,"name":name,"line":line,"type":typ,"id":"a;s","steps":steps}
    if before: e["before"]=before
    if after: e["after"]=after
    return e
def feat(els,bg=None): 
    return [{"uri":"a.feature","id":"a","keyword":"Feature","name":"A","line":1,"elements":els}]
cases={
 # attempt1 fails at unannotated step 2; final fails at annotated step 1
 "z1_prior_other_step":[sc([st("x","passed"),st("y","failed",line=5)]), sc([st("x","failed"),st("y","skipped",line=5)])],
 "z2_prior_before_hook":[sc([st("x","skipped"),st("y","skipped",line=5)],before=[hook("failed")]), sc([st("x","failed"),st("y","skipped",line=5)])],
 "z3_prior_afterstep":[sc([st("x","passed",after=[hook("failed")]),st("y","skipped",line=5)]), sc([st("x","failed"),st("y","skipped",line=5)])],
 "z4_prior_after_hook":[sc([st("x","failed"),st("y","skipped",line=5)],after=[hook("failed")]), sc([st("x","failed"),st("y","skipped",line=5)])],
 "z5_prior_undefined":[sc([st("x","undefined"),st("y","skipped",line=5)]), sc([st("x","failed"),st("y","skipped",line=5)])],
 "z6_prior_same_ok":[sc([st("x","failed"),st("y","skipped",line=5)]), sc([st("x","failed"),st("y","skipped",line=5)])],
 "z7_prior_dup_text":[sc([st("x","passed"),st("x","failed",line=5)]), sc([st("x","failed"),st("x","skipped",line=5)])],
 "z8_prior_fewer_steps":[sc([st("y","failed",line=5)]), sc([st("x","failed"),st("y","skipped",line=5)])],
 "z9_prior_pass_final_known":[sc([st("x","failed"),st("y","skipped",line=5)]), sc([st("x","passed"),st("y","failed",line=5)])],
 "z10_prior_ambig":[sc([st("x","ambiguous"),st("y","skipped",line=5)]), sc([st("x","failed"),st("y","skipped",line=5)])],
 "z11_prior_pending_then_hook":[sc([st("x","pending"),st("y","skipped",line=5)],after=[hook("passed")]), sc([st("x","failed"),st("y","skipped",line=5)])],
}
for k,v in cases.items():
    json.dump(feat(v),open(os.path.join(D,k+".json"),"w"))
# background variants
bg=lambda s: {"keyword":"Background","name":"","line":2,"type":"background","steps":s}
json.dump([{"uri":"a.feature","id":"a","keyword":"Feature","name":"A","line":1,"elements":[bg([st("b","passed",line=2),st("b","failed",line=2)]),sc([st("x","skipped")]), bg([st("b","failed",line=2),st("b","skipped",line=2)]),sc([st("x","skipped")])]}],open(os.path.join(D,"zb1_bg_dup_other_pos.json"),"w"))
json.dump({"version":2,"annotations":[{"scenario":"a-L3","kind":"known","ref":"K","title":"t","step":1,"stepText":"x"}]},open(os.path.join(D,"k1.opt"),"w"))
json.dump({"version":2,"annotations":[{"scenario":"a-L3","kind":"known","ref":"K","title":"t","backgroundStep":1,"stepText":"b"}]},open(os.path.join(D,"kb1.opt"),"w"))
json.dump({"version":2,"annotations":[{"scenario":"a-L3","kind":"known","ref":"K","title":"t","step":1,"stepText":"x","observed":True}]},open(os.path.join(D,"k1obs.opt"),"w"))

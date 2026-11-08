#!/usr/bin/env python3
"""Disable outcome rules in an isolated source copy; runtime safeguards must refuse.
Adapted from reviewer X, using repository fixtures. Cross-input reruns are intentionally absent.
"""
import ast, pathlib, shutil, subprocess, os, json, tempfile, hashlib
root=pathlib.Path(__file__).resolve().parents[2]; review=root/'testdata'
mutations={'M1-retry-rescue': [('internal/cucumbermodel/status.go', 'if raw != StatusPassed && priorFailure {', 'if false && raw != StatusPassed && priorFailure {')], 'M2-known-isolation': [('internal/cucumbermodel/status.go', 'if matched && allowed {', 'if matched && (allowed || true) {')], 'M3a-bg-fold-enh': [('internal/cucumbermodel/fold.go', '{{sc.Before, "hook"}, {background, "background"}, {sc.Steps, "own"}, {sc.After, "hook"}} {\n\t\tif err := appendSteps', '{{sc.Before, "hook"}, {nil, "background"}, {sc.Steps, "own"}, {sc.After, "hook"}} {\n\t\tif err := appendSteps')], 'M3b-bg-fold-plain': [('internal/cucumbermodel/fold.go', '{{sc.Before, "hook"}, {background, "background"}, {sc.Steps, "own"}, {sc.After, "hook"}} {\n\t\t\t\tif err := add', '{{sc.Before, "hook"}, {nil, "background"}, {sc.Steps, "own"}, {sc.After, "hook"}} {\n\t\t\t\tif err := add')], 'M4a-gate-eval': [('report/run.go', 'if g.Pending && s.Counts["pending"] > 0 {', 'if false && g.Pending && s.Counts["pending"] > 0 {'), ('report/run.go', 'if g.Flaky && s.Counts["flaky"] > 0 {', 'if false && g.Flaky && s.Counts["flaky"] > 0 {'), ('report/run.go', 'if g.Empty && s.ObservedScenarios == 0 {', 'if false && g.Empty && s.ObservedScenarios == 0 {')], 'M4b-gate-verdict': [('report/run.go', 'if len(summary.GateFailures) > 0 {\n\t\tsummary.Computed = Verdict{', 'if false {\n\t\tsummary.Computed = Verdict{')], 'M4c-flaky-count': [('report/run.go', 'if modelmeta.Decide(sc, nil, nil).Flaky && !ss.RunHook {', 'if false && modelmeta.Decide(sc, nil, nil).Flaky && !ss.RunHook {')], 'M5-runner-overlay': [('report/model.go', 'if modelmeta.RunnerFailed(features) && p.Verdict.Tone != ToneFail {', 'if false {'), ('report/run.go', 'if g.Unexpected && (s.Counts["failed"] > 0 || s.RunnerFailed || hookFailure) {', 'if g.Unexpected && (s.Counts["failed"] > 0 || hookFailure) {')], 'M6-around': [('internal/cucumberinput/stream.go', 'sc.After = append(sc.After, around...)', '_ = around')], 'M8a-zero-tone': [('report/model.go', 'return Verdict{Label: "No scenarios executed", Tone: ToneInfo}', 'return Verdict{Label: "No scenarios executed", Tone: ToneOK}')], 'M8b-zero-as-passed': [('report/model.go', 'if len(all) == 0 || (len(counts) == 0 && overall == "passed") {', 'if false {')], 'N1-cross-input-refusal': [('report/run.go', 'if err := rejectRepeatedInputs(inputs); err != nil {', 'if err := rejectRepeatedInputs(inputs); false && err != nil {')], 'N3-inferred-retry': [('internal/cucumberinput/messages.go', 'if ordinals[start.TestCaseID] < totals[start.TestCaseID] && modelmeta.ExecutionStatus(sc) == StatusPassed {', 'if false && ordinals[start.TestCaseID] < totals[start.TestCaseID] && modelmeta.ExecutionStatus(sc) == StatusPassed {')], 'N4-case-attach': [('internal/cucumberinput/messages.go', 'if strings.HasPrefix(key, id+"\\x00") {', 'if key == id+"\\x00" {')], 'N5a-known-prior': [('internal/cucumbermodel/status.go', '(st.Result.Status != StatusFailed || !positions[i+1]) {\n\t\t\t\t\tallowed = false', '(st.Result.Status != StatusFailed || !positions[i+1]) {\n\t\t\t\t\tallowed = allowed')], 'N5b-known-matchpos': [('internal/cucumbermodel/status.go', 'st.stepState.OwnIndex == before.stepState.OwnIndex && st.Name', 'st.Name'), ('internal/cucumbermodel/status.go', 'if st.stepState.Origin == "background" && background != priorBackground {', 'if false {')], 'N5c-known-reported': [('internal/cucumbermodel/status.go', 'if status != StatusPassed && status != StatusSkipped {\n\t\t\t\tallowed = false', 'if status != StatusPassed && status != StatusSkipped {\n\t\t\t\tallowed = allowed')], 'N6-zero-runner': [('internal/cucumberinput/messages.go', 'if runFailed && len(out.Attempts) == 0 {', 'if false && runFailed && len(out.Attempts) == 0 {')], 'N7-alias': [('report/model.go', 'for from, to := range modelmeta.FeatureMetadata(&f).BackgroundSlots {\n\t\t\tif opt.Analysis != nil || modelmeta.FeatureMetadata(&f).ReportID != "" {', 'for from, to := range modelmeta.FeatureMetadata(&f).BackgroundSlots {\n\t\t\tif false {'), ('report/model.go', 'if _, live := stepCount[fids[i]+from]; live {', 'if _, live := stepCount[fids[i]+from]; live && false {')], 'N2-stream-unresolved': [('report/raw_outcomes.go', 'if !stream.Failed {', 'if true || !stream.Failed {')]}
combined=mutations.pop('M4a-gate-eval')
for label,edit in zip(['M4p-pending','M4f-flaky','M4e-empty'],combined):mutations[label]=[edit]
mutations['M4u-undefined']=[('report/run.go','if g.Undefined && (s.Counts["undefined"]+s.Counts["ambiguous"] > 0) {','if false {')]
mutations['M4x-unexpected']=[('report/run.go','if g.Unexpected && (s.Counts["failed"] > 0 || s.RunnerFailed || hookFailure) {','if false {')]
mutations['M4i-incomplete']=[('report/run.go','if g.Incomplete && s.Completeness != "complete" {','if false {')]
mutations['M4m-baseline']=[('report/run.go','if (g.NewFailures || g.MaxRegressionPercent > 0) && s.Comparison == nil && !g.AllowFirstRun {','if false {')]
mutations['M4n-new-failures']=[('report/run.go','if g.NewFailures && len(s.Comparison.NewFailures) > 0 {','if false {')]
mutations['M4d-duration']=[('report/run.go','if x.DurationComparable && durationRegressed(x.DurationDelta, base, g) {','if false && x.DurationComparable && base > 0 {')]
mutations['M4b-gate-verdict']=[('report/run.go','if !redundant {','if false {')]
mutations['S2-strict-flaky']=[('report/raw_outcomes.go','stream.Unexplained = append(stream.Unexplained, "runner may have failed for strict-flaky")','_ = 0')]
mutations['S4-stream-signal']=[('report/raw_outcomes.go','stream.Cases[key] = append(stream.Cases[key], rawExecution(a.Scenario))\n\t\t\tstream.Failed = stream.Failed || a.RunnerFailed','stream.Cases[key] = append(stream.Cases[key], rawExecution(a.Scenario))')]
mutations['FR-background-refusal']=[('report/input_collisions.go','strings.EqualFold(s.Type, "background") || ',''),('report/raw_outcomes.go','strings.EqualFold(sc.Type, "background") || ','')]
mutations['S6-own-step-guard']=[('report/raw_outcomes.go','if last.NoOwnSteps || len(last.Results) == 0 {','if len(last.Results) == 0 {')]
# Outcome-rule edits require an explicit inventory review, not just a green old suite.
inventory=json.loads((root/'internal/surfacecheck/outcome-rules.json').read_text())
for file,digest in inventory['sources'].items():
 if hashlib.sha256((root/file).read_bytes()).hexdigest()!=digest:
  raise SystemExit('Outcome rules changed without reviewing mutant inventory: '+file)
for folder in inventory['directories']:
 for f in sorted((root/folder).glob('*.go')):
  if not f.name.endswith('_test.go') and f.relative_to(root).as_posix() not in inventory['sources']:
   raise SystemExit('Outcome rule source is not pinned in the mutant inventory: '+f.relative_to(root).as_posix())
mutations['P1-unexplained']=[('report/raw_outcomes.go','if len(stream.Unexplained) > 0 {','if false {')]
mutations['P2-soft']=[('report/raw_outcomes.go','if status == "pending" || status == "undefined" || status == "unknown" {','if false && (status == "pending" || status == "undefined" || status == "unknown") {')]
mutations['P4-unaccounted']=[('report/raw_outcomes.go','return !explained','return false && !explained')]
# Known-issue matching: the floor re-derives coverage from annotations and raw step text.
# Issue policies: the floor re-derives exclusions from the raw RunConfig issues.
mutations['K4-issue-policy']=[('report/run.go','a.PolicyReason = reason','_ = reason')]
mutations['K1-stepText']=[('report/model.go','if clean(sc.Steps[annotationPosition(a)-1].Name) != a.StepText {','if false {')]
# Redaction masks only published text: the decision keeps a mismatch that masking
# collapsed, and decides issue policies on the unmasked configuration. The floor
# checks both against its own pre-redaction snapshot.
mutations['K5-redacted-steptext']=[('report/run.go','a.StepText = name + " (differs before redaction)"','_ = name')]
mutations['K7-redacted-retry']=[('report/run.go','\t\tfor prior.Name == cur.Name && prior.Keyword == cur.Keyword {','\t\tfor false && prior.Name == cur.Name {')]
mutations['K6-redacted-policy']=[('report/run.go','applyIssuePolicies(features, ids, &opt, policyCfg)','applyIssuePolicies(features, ids, &opt, cfg)')]
# Redundant defensive branches are unit-mutated; they cannot independently make
# a real capture greener. Do not misreport an unchanged CLI run as a floor kill.
unit_only={
 'U1-run-hook':[('report/raw_outcomes.go','stream.Unexplained = append(stream.Unexplained, "failed run hook")','_ = 0')],
 'U2-omitted':[('report/raw_outcomes.go','stream.Unexplained = append(stream.Unexplained, "runner omitted hook or execution results")','_ = 0')],
 'U3-feature-signal':[('report/raw_outcomes.go','stream.Failed = stream.Failed || model.ScenarioMetadata(&s).RunnerFailed','_ = 0')],
 'U4-missing-final':[('report/raw_outcomes.go','v, ok := final[key]\n\t\tif !ok {\n\t\t\treturn true','v, ok := final[key]\n\t\tif !ok {\n\t\t\tcontinue')],
 'U5-final-claim':[('report/raw_outcomes.go','v := evidence.Final{Status: statuses[key]}','v := evidence.Final{Status: "known"}')],
 # Floor-side snapshot wiring: each alone leaves the decision correct (a backstop)
 # or refuses a legitimate run (U8, and U9: masking narrows coverage, a false
 # red the floor does not judge); TestRound19* kill them.
 'U6-floor-masked-annotations':[('report/evidence.go','annotations = orig.annotations','_ = orig.annotations')],
 'U7-floor-masked-scenario':[('report/evidence.go','if raw, ok = orig.scenarios[sc.ID]; !ok || len(raw.Steps) != len(original.sc.Steps) {','if _, ok = orig.scenarios[sc.ID]; !ok {')],
 'U9-redacted-narrowing':[('report/run.go','\t\tcase matched[i] && clean(a.StepText) != name:\n\t\t\ta.StepText = name\n','')],
 'U8-floor-masked-policy':[('report/run.go','checkScenarioOutcomes(features, classified, &policyCfg, original)','checkScenarioOutcomes(features, classified, &cfg, original)')],
}
# Reviewer inventory r17 (fixtures F4/mutants4.py). S1=U1, S3=U2 and K3=M2 above use
# the same edits. These singles change no real capture alone (another layer decides
# the same verdict), and each floor rule alone is a backstop that must not change a
# verdict. The doubles disable a floor rule together with the production rule it
# guards, which defeats the runtime by construction (k23/k24 become 0/warn,
# k09/k13/k18 0/ok). All of them are killed by the Go regression tests.
CR_OFF=('internal/evidence/audit.go','if !claimedFailure && (causes > 0 || covered == 0) {','if false && !claimedFailure && (causes > 0 || covered == 0) {')
S2=('report/raw_outcomes.go','stream.Unexplained = append(stream.Unexplained, "runner may have failed for strict-flaky")','_ = 0')
S4=mutations['S4-stream-signal'][0]
R1=('internal/cucumberinput/messages.go','runFailed = !*v.Success','runFailed = false && !*v.Success')
R2=('internal/evidence/evidence.go','if parent == Fold("testRunFinished") && k == Fold("success") && v == false {','if false && parent == Fold("testRunFinished") && k == Fold("success") && v == false {')
RAWF=('internal/evidence/messages.go','if !*event.Finish.Success {\n\t\t\tm.failed = true','if false && !*event.Finish.Success {\n\t\t\tm.failed = true')
unit_only.update({
 'S4b-runhook-stream-failed':[('report/raw_outcomes.go','stream.Failed = stream.Failed || a.RunnerFailed\n\t\t\t\tfor','_ = 0\n\t\t\t\tfor')],
 'R1-parser-success':[R1],
 'R2-rawscan-success':[R2],
 'F1-checkrunner-off':[CR_OFF],
 'F2-checkrunner-flaky':[('internal/evidence/audit.go','if len(attempts) > 1 && allPass {\n\t\t\tcauses++','if false && len(attempts) > 1 && allPass {\n\t\t\tcauses++')],
 'F3-checkrunner-soft':[('internal/evidence/audit.go','soft = soft || s == "undefined" || s == "pending" || s == "unknown"','soft = soft')],
 'F4-checkrunner-covered':[('internal/evidence/audit.go','if !claimedFailure && (causes > 0 || covered == 0) {','if !claimedFailure && (causes > 0) {')],
 'F5-checkcases-incomplete':[('internal/evidence/audit.go','if (final.NoOwnSteps || final.MissingResults) && (shown.Status','if false && (final.NoOwnSteps || final.MissingResults) && (shown.Status')],
 'F6-messages-check':[('internal/evidence/messages.go','if m.failed && !runnerFailed {','if false && m.failed && !runnerFailed {')],
 'E1-checkrunner+S2':[CR_OFF,S2],
 'E2-checkrunner+S4':[CR_OFF,S4],
 'E3-checkrunner-flaky+S2':[('internal/evidence/audit.go','if len(attempts) > 1 && allPass {\n\t\t\tcauses++','if false && len(attempts) > 1 && allPass {\n\t\t\tcauses++'),S2],
 'E7-checkrunner+P1':[CR_OFF,mutations['P1-unexplained'][0]],
 'R4-parser+rawscan+audit':[R1,R2,RAWF],
 'R5-rawor+parser+audit':[('internal/cucumberinput/messages.go','runFailed = runFailed || rawFloor.RunnerFailed','runFailed = runFailed'),R1,RAWF],
})
mutations.update(unit_only)
work=pathlib.Path(tempfile.mkdtemp(prefix='cr-r14-mut-'));src=work/'src'
shutil.copytree(root,src,ignore=shutil.ignore_patterns('.git','dist','graphify-out','testdata','__pycache__','node_modules'))
# Mutants must fail the ordinary Go regression suite as well as the runtime floor.
shutil.copytree(review,src/'testdata',copy_function=shutil.copy2)
# Minifier embeds its testdata only in tests; report production embeds assets/templates, copied above.
env=dict(os.environ,GOPROXY='off',GOFLAGS='-mod=readonly')
C=review/'captured/ruby-9.2.1-r12'; R=review/'round13/captured-ruby-9.2.1'; T=review/'round13'
def case(name,files,extra=(),plain=False):return(name,files,list(extra),plain)
cases=[case('retryfinal-json',[C/'retryfinal/run.json']),case('retryfinal-msg',[C/'retryfinal/run.ndjson'])]
for n in ['final_pending','final_skipped','success_false_allpass']:cases.append(case(n,[review/'round14/messages'/f'{n}.ndjson']))
for n in ['background','retry-background','zero-tags']:
 f=(C if n=='background' else R)/n/'run.json';cases.extend([case(n+'-enh',[f]),case(n+'-plain',[f],plain=True)])
for n in ['k5_two_failed','k6_afterstep_known']:
 cases.append(case(n,[T/'known'/f'{n}.json'],['-options',str(T/'known'/f'{n}.opt')]))
cases.append(case('known-hook',[review/'round14/known-hook/cucumber.json'],['-options',str(review/'round14/known-hook/options.json')]))
for gate,fixture in [('undefined',R/'strict-undefined-only/run.json'),('pending',C/'pendonly-strict/run.json'),('flaky',C/'flakyonly-strict/run.json'),('empty',R/'zero-tags/run.json')]:
 cfg=work/(gate+'.json');cfg.write_text(json.dumps({'gates':{gate:True}}));cases.append(case('gate-'+gate,[fixture],['-run-config',str(cfg)]))
cases.append(case('around-json',[R/'around-after-only/run.json']))
for fmt in ['json','ndjson']:
 cases.append(case('repeat-'+fmt,[review/f'round14/captured/a1-rerun-allpass/run{i}.{fmt}' for i in [1,2]]))
(work/'zero-failed.ndjson').write_text('{"testRunFinished":{"success":false}}\n')
cases += [case('strict-runner',[review/'round15/captured/g3-same-strict/run.ndjson']),case('inferred',[review/'round14/captured/b4b-around-after-fail-retry/run.ndjson']),case('case-attach',[review/'round14/captured/b5-around-attach-log/run.ndjson']),case('zero-runner',[work/'zero-failed.ndjson']),case('alias',[review/'captured/cucumber-ruby-9.2.1-retry/retry.json'])]
for name,opt in [('z11_prior_pending_then_hook','k1.opt'),('z7_prior_dup_text','k1.opt'),('zb1_bg_dup_other_pos','kb1.opt')]:
 cases.append(case(name,[review/'round15/known'/f'{name}.json'],['-options',str(review/'round15/known'/opt)]))
cases.append(case('reported',[review/'round15/known/n5c.json'],['-options',str(review/'round15/known/k1.opt'),'-run-config',str(review/'round15/known/n5c-rc.json')]))
cases.append(case('strict-flaky-known',[review/'round16/F3/cap/k24-known-z-strictflaky/run.ndjson'],['-options',str(review/'round16/F3/adv/knownz.opt')]))
for ext in ['json','ndjson']:
 cases.append(case('real-disjoint-'+ext,[review/('round16/G3/cap/a.'+ext),review/('round16/G3/cap/b.'+ext)]))
cases.append(case('empty-with-known',[review/'round16/independent/empty-with-known.ndjson'],['-options',str(review/'round16/independent/known.opt')]))
raw=[json.loads(line) for line in (review/'round16/F3/cap/k25-known-z-only/run.ndjson').read_text().splitlines() if line.strip()]
for event in raw:
 result=event.get('testStepFinished',{}).get('testStepResult',{})
 if result.get('status')=='PASSED':result['status']='PENDING';break
soft=work/'soft-known.ndjson';soft.write_text('\n'.join(map(json.dumps,raw))+'\n')
cases.append(case('soft-known',[soft],['-options',str(review/'round16/F3/adv/knownz.opt')]))
for cap in ['k22-sp-known-z-strict','k23-known-z-flaky-x','k24-known-z-strictflaky','k25-known-z-only']:
 policy={'k22-sp-known-z-strict':['-run-config',str(review/'round16/F3/adv/strict.json')],'k24-known-z-strictflaky':['-run-config',str(review/'round16/F3/adv/sflaky.json')]}.get(cap,[])
 for ext in ['json','ndjson']:
  cases.append(case(f'knownwrong-{cap[:3]}-{ext}',[review/'round16/F3/cap'/cap/f'run.{ext}'],['-options',str(review/'round16/F3/adv/knownwrong.opt')]+(policy if ext=='json' else [])))
issue_policies={'env':{'environment':'prod','issues':[{'ref':'K','environments':['staging']}]},'expired':{'recordedAt':'2026-03-02T10:00:00Z','issues':[{'ref':'K','expires':'2026-03-01'}]},'signature':{'issues':[{'ref':'K','errorPattern':'^not this error$'}]}}
for cap in ['k22-sp-known-z-strict','k23-known-z-flaky-x','k24-known-z-strictflaky','k25-known-z-only']:
 gates={'k22-sp-known-z-strict':{'pending':True,'undefined':True},'k24-known-z-strictflaky':{'flaky':True}}.get(cap)
 for pname,pol in issue_policies.items():
  for ext in ['json','ndjson']:
   cfg=dict(pol)
   if gates and ext=='json':cfg['gates']=gates
   cfgfile=work/f'issue-{pname}-{cap[:3]}-{ext}.json';cfgfile.write_text(json.dumps(cfg))
   cases.append(case(f'issue-{pname}-{cap[:3]}-{ext}',[review/'round16/F3/cap'/cap/f'run.{ext}'],['-options',str(review/'round16/F3/adv/knownz.opt'),'-run-config',str(cfgfile)]))
R19=review/'round19'
for ext in ['json','ndjson']:
 cases.append(case('redacted-steptext-'+ext,[R19/f'known-redaction.{ext}'],['-options',str(R19/f'known-{ext}-wrong.opt'),'-run-config',str(R19/'redact-steptext.json')]))
 cases.append(case('redacted-environment-'+ext,[R19/f'known-redaction.{ext}'],['-options',str(R19/f'known-{ext}-right.opt'),'-run-config',str(R19/'redact-environment.json')]))
cases.append(case('redacted-retry-json',[review/'round20/retry-names.json'],['-options',str(review/'round20/retry-names.opt'),'-run-config',str(R19/'redact-steptext.json')]))
cases.append(case('unaccounted-wip',[review/'round16/F3/cap/k19-wip-pass/run.ndjson']))
reference=work/'reference'
subprocess.run(['go','build','-o',str(reference),'./cmd/cucumber-report'],cwd=root,env=env,check=True)
def minimal(status,duration):return [{'uri':'gate.feature','name':'Gate','elements':[{'name':'case','type':'scenario','line':2,'steps':[{'name':'step','result':{'status':status,'duration':duration}}]}]}]
basefile=work/'base.json';basefile.write_text(json.dumps(minimal('passed',100000000)))
baseout=work/'baseline';baseout.mkdir()
subprocess.run([str(reference),'-enhanced','-o',str(baseout),str(basefile)],cwd=root,env=env,check=True,capture_output=True)
baseline=next(baseout.glob('html-*/summary.json'))
for name,policy,status,duration in [('incomplete',{'incomplete':True},'passed',100000000),('baseline',{'newFailures':True},'passed',100000000),('new',{'newFailures':True},'failed',100000000),('duration',{'maxRegressionPercent':20},'passed',400000000)]:
 cfg=work/(name+'-config.json');cfg.write_text(json.dumps({'gates':policy}))
 data=work/(name+'-input.json');data.write_text(json.dumps(minimal(status,duration)))
 extra=['-run-config',str(cfg)]
 if name in ('new','duration'):extra+=['-baseline',str(baseline)]
 cases.append(case('gate-'+name,[data],extra))
print('SOURCE',work,flush=True)
expected={'M1':['retryfinal-json','retryfinal-msg','final_pending','final_skipped'],'M2':['k5_two_failed','known-hook'],'M3a':['background-enh'],'M3b':['background-plain'],'M4a':['gate-pending','gate-flaky','gate-empty'],'M4b':['gate-pending','gate-empty'],'M4c':['gate-flaky'],'M5':['success_false_allpass'],'M6':['around-json'],'M8a':['zero-tags-enh','zero-tags-plain'],'M8b':['zero-tags-enh','zero-tags-plain']}
expected.update({'M4i':['gate-incomplete'],'M4m':['gate-baseline'],'M4n':['gate-new'],'M4d':['gate-duration']})
expected.update({'M4p':['gate-pending'],'M4f':['gate-flaky'],'M4e':['gate-empty'],'M4u':['gate-undefined'],'M4x':['retryfinal-json','retryfinal-msg']})
expected.update({'N1':['repeat-json','repeat-ndjson'],'N2':['strict-runner'],'N3':['inferred'],'N4':['case-attach'],'N5a':['z11_prior_pending_then_hook'],'N5b':['z7_prior_dup_text','zb1_bg_dup_other_pos'],'N5c':['reported'],'N6':['zero-runner'],'N7':['alias']})
expected['S6']=['empty-with-known']
expected['K4']=[f'issue-{p}-k2{n}-{ext}' for p in ['env','expired','signature'] for n in '2345' for ext in ['json','ndjson']]
expected['K5']=['redacted-steptext-json','redacted-steptext-ndjson']
expected['K7']=['redacted-retry-json']
expected['K6']=['redacted-environment-json','redacted-environment-ndjson']
expected['K1']=[f'knownwrong-k2{n}-{ext}' for n in '2345' for ext in ['json','ndjson']]
expected.update({'P1':['strict-flaky-known'],'P2':['soft-known'],'P4':['unaccounted-wip']})
expected.update({'S2':['strict-flaky-known'],'S4':['strict-runner'],'FR':['real-disjoint-json','real-disjoint-ndjson']})
failures=[]
for mid,edits in mutations.items():
 originals={}
 try:
  for f,old,new in edits:
   p=src/f;s=p.read_text();originals.setdefault(p,s)
   if s.count(old)!=1:raise RuntimeError(f'{mid} pattern count {s.count(old)}: {old}')
   p.write_text(s.replace(old,new))
  b=work/'mutant';r=subprocess.run(['go','build','-o',str(b),'./cmd/cucumber-report'],cwd=src,env=env,capture_output=True,text=True)
  if r.returncode:raise RuntimeError(r.stderr)
  test=subprocess.run(['go','test','./cmd/cucumber-report','./report','./internal/cucumberinput','./internal/cucumbermodel','./internal/evidence','-run','TestRound|TestRoot|TestRunner|TestRun|TestKnown|TestOutcome|TestRaw','-count=1'],cwd=src,env=env,capture_output=True,text=True,timeout=180)
  (work/(mid+'-go-test.log')).write_text(test.stdout+test.stderr)
  if test.returncode==0 or '--- FAIL:' not in test.stdout:
   failures.append((mid,'Go regression tests did not kill mutant',test.stdout[-1000:],test.stderr[-500:]))
   print(mid,'GO TEST KILL FAILED',flush=True)
  else:print(mid,'KILLED by Go regression tests',flush=True)
  if mid in unit_only:
   print(mid,'DEFENSIVE UNIT KILL (no runtime change claimed)',flush=True);continue
  key=mid.split('-')[0]
  for name,files,extra,plain in cases:
   if name not in expected[key]:continue
   out=work/'out';shutil.rmtree(out,ignore_errors=True);out.mkdir()
   args=[str(b),'-o',str(out)]
   if plain:
    inp=work/'input';inp.mkdir(exist_ok=True);shutil.copyfile(files[0],inp/'cucumber.json');args+=[str(inp)]
   else:args+=['-enhanced','-format','messages' if files[0].suffix=='.ndjson' else 'json']+([] if name=='gate-new' else ['-fail-on-unexpected'])+extra+list(map(str,files))
   r=subprocess.run(args,cwd=root,env=env,capture_output=True,text=True,timeout=30)
   text=(r.stdout+r.stderr).strip().replace('\n',' ')[:400]
   good=r.returncode==1 and (any(marker in text for marker in ("evidence floor", "final attempt differs", "non-passing result in an unsupported execution field")) or (mid == "N4-case-attach" and "stream incomplete: execution events were not matched" in text))
   if mid.startswith('FR-'):
    # A false-refusal mutant is killed by the positive acceptance contract.
    good=(r.returncode==1 and 'repeated scenarios' in text) if name.endswith('json') and not name.endswith('ndjson') else r.returncode==3
   print(mid,name,'PASS' if good else 'FAIL',r.returncode,text,flush=True)
   if not good:failures.append((mid,name,r.returncode))
 except Exception as e:print('ERROR',mid,str(e),flush=True);failures.append((mid,str(e)))
 finally:
  for p,s in originals.items():p.write_text(s)
print('FAILURES',failures,flush=True)

if not failures: shutil.rmtree(work)
raise SystemExit(bool(failures))

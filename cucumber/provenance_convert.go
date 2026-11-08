package cucumber

import (
	"reflect"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

func publicFeatureProvenance(v *modelmeta.FeatureState) *FeatureProvenance {
	if reflect.DeepEqual(*v, modelmeta.FeatureState{}) {
		return nil
	}
	out := &FeatureProvenance{OrphanBackground: v.OrphanBackground, BackgroundSlots: v.BackgroundSlots, IdentityCatalog: v.IdentityCatalog, ReportID: v.ReportID, SyntheticRunHooks: v.SyntheticRunHooks}
	return out
}

func internalFeatureProvenance(v *FeatureProvenance) modelmeta.FeatureState {
	if v == nil {
		return modelmeta.FeatureState{}
	}
	out := modelmeta.FeatureState{OrphanBackground: v.OrphanBackground, BackgroundSlots: v.BackgroundSlots, IdentityCatalog: v.IdentityCatalog, ReportID: v.ReportID, SyntheticRunHooks: v.SyntheticRunHooks}
	return out
}

func publicStepProvenance(v *modelmeta.StepState) *StepProvenance {
	if reflect.DeepEqual(*v, modelmeta.StepState{}) {
		return nil
	}
	out := &StepProvenance{HookKind: v.HookKind, HookStep: v.HookStep, Origin: v.Origin, OwnIndex: v.OwnIndex}
	return out
}

func internalStepProvenance(v *StepProvenance) modelmeta.StepState {
	if v == nil {
		return modelmeta.StepState{}
	}
	out := modelmeta.StepState{HookKind: v.HookKind, HookStep: v.HookStep, Origin: v.Origin, OwnIndex: v.OwnIndex}
	return out
}

func publicScenarioProvenance(v *modelmeta.ScenarioState) *ScenarioProvenance {
	if reflect.DeepEqual(*v, modelmeta.ScenarioState{}) {
		return nil
	}
	out := &ScenarioProvenance{RawCaseAudit: v.RawCaseAudit, RawRunnerAudit: v.RawRunnerAudit, RunnerFailed: v.RunnerFailed, InferredRetryFailure: v.InferredRetryFailure, CaseAttachments: v.CaseAttachments, MissingStepResults: v.MissingStepResults, EnhancedFolded: v.EnhancedFolded, PlainFolded: v.PlainFolded, NavigationSuffix: v.NavigationSuffix, IdentityNameCount: v.IdentityNameCount, BackgroundUnresolved: v.BackgroundUnresolved}
	for _, s := range v.ReportedAttempts {
		out.ReportedAttempts = append(out.ReportedAttempts, Status(s))
	}
	for _, e := range v.OutcomeFloor {
		x := ExecutionEvidence{Own: e.Own}
		for _, s := range e.Results {
			x.Results = append(x.Results, Status(s))
		}
		out.OutcomeFloor = append(out.OutcomeFloor, x)
	}
	out.AttemptBackground = publicSteps(v.AttemptBackground, 0)
	for _, s := range v.PreviousAttempts {
		out.PreviousAttempts = append(out.PreviousAttempts, publicScenario(s))
	}
	return out
}

func internalScenarioProvenance(v *ScenarioProvenance) modelmeta.ScenarioState {
	if v == nil {
		return modelmeta.ScenarioState{}
	}
	out := modelmeta.ScenarioState{RawCaseAudit: v.RawCaseAudit, RawRunnerAudit: v.RawRunnerAudit, RunnerFailed: v.RunnerFailed, InferredRetryFailure: v.InferredRetryFailure, CaseAttachments: v.CaseAttachments, MissingStepResults: v.MissingStepResults, EnhancedFolded: v.EnhancedFolded, PlainFolded: v.PlainFolded, NavigationSuffix: v.NavigationSuffix, IdentityNameCount: v.IdentityNameCount, BackgroundUnresolved: v.BackgroundUnresolved}
	for _, s := range v.ReportedAttempts {
		out.ReportedAttempts = append(out.ReportedAttempts, modelmeta.Status(s))
	}
	for _, e := range v.OutcomeFloor {
		x := modelmeta.ExecutionEvidence{Own: e.Own}
		for _, s := range e.Results {
			x.Results = append(x.Results, modelmeta.Status(s))
		}
		out.OutcomeFloor = append(out.OutcomeFloor, x)
	}
	out.AttemptBackground = internalSteps(v.AttemptBackground, 0)
	for _, s := range v.PreviousAttempts {
		out.PreviousAttempts = append(out.PreviousAttempts, internalScenario(s))
	}
	return out
}

package report

import (
	"fmt"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

// foldScenarioSteps is shared by legacy scenarios and Messages attempts. Input
// adapters retain origins; only this function assigns own-step annotation indices.
func foldScenarioSteps(sc cucumber.Scenario, background []cucumber.Step) (cucumber.Scenario, error) {
	return modelmeta.FoldScenarioSteps(sc, background)
}

func ownStepMap(sc cucumber.Scenario) map[int]int {
	out := map[int]int{}
	for i, st := range sc.Steps {
		if modelmeta.StepMetadata(&st).Origin == "own" || (modelmeta.ScenarioMetadata(&sc).PlainFolded && modelmeta.StepMetadata(&st).OwnIndex > 0) {
			out[modelmeta.StepMetadata(&st).OwnIndex] = i + 1
		}
	}
	return out
}

func enhancedSteps(sc cucumber.Scenario) bool {
	return len(sc.Steps) > 0 && modelmeta.StepMetadata(&sc.Steps[0]).Origin != ""
}

func annotationStepCount(sc cucumber.Scenario) int {
	if enhancedSteps(sc) {
		return len(ownStepMap(sc))
	}
	return len(sc.Steps)
}

func declaredPosition(sc cucumber.Scenario, index int) int {
	if enhancedSteps(sc) {
		return ownStepMap(sc)[index]
	}
	return index
}

func annotationPosition(a *annotationData) int {
	if a.ExecutionStep > 0 {
		return a.ExecutionStep
	}
	return a.Step
}

func stepLocation(sc cucumber.Scenario, index int) string {
	if index < 1 || index > len(sc.Steps) || !enhancedSteps(sc) {
		return fmt.Sprintf("step %d", index)
	}
	st := sc.Steps[index-1]
	if modelmeta.ScenarioMetadata(&sc).PlainFolded && modelmeta.StepMetadata(&st).OwnIndex > 0 {
		return fmt.Sprintf("step %d", modelmeta.StepMetadata(&st).OwnIndex)
	}
	if modelmeta.StepMetadata(&st).Origin == "own" {
		return fmt.Sprintf("step %d", modelmeta.StepMetadata(&st).OwnIndex)
	}
	if modelmeta.StepMetadata(&st).Origin == "background" {
		return "a Background step"
	}
	if meta := modelmeta.StepMetadata(&st); meta.HookKind != "" && meta.HookStep > 0 {
		return fmt.Sprintf("a hook %s step %d", meta.HookKind, meta.HookStep)
	}
	if index > 1 && modelmeta.StepMetadata(&sc.Steps[index-2]).Origin == "background" {
		return "a hook after a Background step"
	}
	if modelmeta.StepMetadata(&st).HookKind == "" {
		previous, following := 0, 0
		for i := index - 2; i >= 0; i-- {
			if m := modelmeta.StepMetadata(&sc.Steps[i]); m.Origin == "own" {
				previous = m.OwnIndex
				break
			}
		}
		for _, next := range sc.Steps[index:] {
			if m := modelmeta.StepMetadata(&next); m.Origin == "own" {
				following = m.OwnIndex
				break
			}
		}
		if previous > 0 && following > 0 {
			return fmt.Sprintf("a hook between steps %d and %d", previous, following)
		}
	}
	for _, next := range sc.Steps[index:] {
		if modelmeta.StepMetadata(&next).Origin == "own" {
			return fmt.Sprintf("a hook before step %d", modelmeta.StepMetadata(&next).OwnIndex)
		}
	}
	for i := index - 2; i >= 0; i-- {
		if modelmeta.StepMetadata(&sc.Steps[i]).Origin == "own" {
			return fmt.Sprintf("a hook after step %d", modelmeta.StepMetadata(&sc.Steps[i]).OwnIndex)
		}
	}
	return "a run hook"
}

func annotationSourcePosition(sc cucumber.Scenario, a Annotation) int {
	if a.BackgroundStep == 0 {
		return declaredPosition(sc, a.Step)
	}
	n := 0
	for i, st := range sc.Steps {
		if modelmeta.StepMetadata(&st).Origin == "background" {
			n++
			if n == a.BackgroundStep {
				return i + 1
			}
		}
	}
	return 0
}

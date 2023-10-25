package clusterlogforwarder

import (
	"strconv"
	"strings"

	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"

	"github.com/openshift/hypershift-logging-operator/api/v1alpha1"
	"github.com/openshift/hypershift-logging-operator/pkg/constants"
)

var (
	InputHTTPServerName = "input-httpserver"
	InputHTTPServerSpec = loggingv1.InputSpec{
		Name: InputHTTPServerName,
		Receiver: &loggingv1.ReceiverSpec{
			HTTP: &loggingv1.HTTPReceiver{
				Format: "kubeAPIAudit",
				ReceiverPort: loggingv1.ReceiverPort{
					Name:       "httpserver",
					Port:       443,
					TargetPort: 8443,
				},
			},
		},
	}
)

// CleanUpClusterLogForwarder removes all the user managed forwarder rules from CLF
func CleanUpClusterLogForwarder(clf *loggingv1.ClusterLogForwarder, keyword string) *loggingv1.ClusterLogForwarder {

	newInputs := clf.Spec.Inputs[:0]
	for _, input := range clf.Spec.Inputs {
		if !strings.Contains(input.Name, keyword) {
			newInputs = append(newInputs, input)
		}
	}

	newOutputs := clf.Spec.Outputs[:0]
	for _, output := range clf.Spec.Outputs {
		if !strings.Contains(output.Name, keyword) {
			newOutputs = append(newOutputs, output)
		}
	}

	newPipelines := clf.Spec.Pipelines[:0]
	for _, ppl := range clf.Spec.Pipelines {
		if !strings.Contains(ppl.Name, keyword) {
			newPipelines = append(newPipelines, ppl)
		}
	}

	newFilters := clf.Spec.Filters[:0]
	for _, fil := range clf.Spec.Filters {
		if !strings.Contains(fil.Name, keyword) {
			newFilters = append(newFilters, fil)
		}
	}

	clf.Spec.Inputs = newInputs
	clf.Spec.Outputs = newOutputs
	clf.Spec.Pipelines = newPipelines
	clf.Spec.Filters = newFilters

	return clf
}

// BuildInputsFromTemplate builds the input array from the template
func BuildInputsFromTemplate(template *v1alpha1.ClusterLogForwarderTemplate,
	clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(clf.Spec.Inputs) < 1 {
		clf.Spec.Inputs = append(clf.Spec.Inputs, InputHTTPServerSpec)
	}

	//if len(template.Spec.Template.Inputs) > 0 {
	//	for _, input := range template.Spec.Template.Inputs {
	//		if !strings.Contains(input.Name, constants.ProviderManagedRuleNamePrefix) {
	//			input.Name = constants.ProviderManagedRuleNamePrefix + "-" + input.Name
	//		}
	//		clf.Spec.Inputs = append(clf.Spec.Inputs, input)
	//	}
	//}

	return clf
}

// BuildOutputsFromTemplate builds the output array from the template
func BuildOutputsFromTemplate(template *v1alpha1.ClusterLogForwarderTemplate,
	clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(template.Spec.Template.Outputs) > 0 {
		for _, output := range template.Spec.Template.Outputs {
			if !strings.Contains(output.Name, constants.ProviderManagedRuleNamePrefix) {
				output.Name = constants.ProviderManagedRuleNamePrefix + "-" + output.Name
			}
			clf.Spec.Outputs = append(clf.Spec.Outputs, output)
		}
	}

	return clf
}

// BuildPipelinesFromTemplate builds the pipeline array from the template
func BuildPipelinesFromTemplate(template *v1alpha1.ClusterLogForwarderTemplate,
	clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(template.Spec.Template.Pipelines) > 0 {
		autoGenName := "auto-generated-name-"
		for x, ppl := range template.Spec.Template.Pipelines {
			if ppl.Name == "" {
				ppl.Name = autoGenName + strconv.Itoa(x)
			}
			if !strings.Contains(ppl.Name, constants.ProviderManagedRuleNamePrefix) {
				ppl.Name = constants.ProviderManagedRuleNamePrefix + "-" + ppl.Name
			}
			clf.Spec.Pipelines = append(clf.Spec.Pipelines, ppl)
		}
	}

	return clf
}

// BuildFiltersFromTemplate builds the filter array from the template
func BuildFiltersFromTemplate(template *v1alpha1.ClusterLogForwarderTemplate,
	clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(template.Spec.Template.Filters) > 0 {
		for _, f := range template.Spec.Template.Filters {
			if !strings.Contains(f.Name, constants.ProviderManagedRuleNamePrefix) {
				f.Name = constants.ProviderManagedRuleNamePrefix + "-" + f.Name
			}
			clf.Spec.Filters = append(clf.Spec.Filters, f)
		}
	}

	return clf
}

// BuildInputsFromHLF builds the CLF inputs from the HLF
func BuildInputsFromHLF(hlf *v1alpha1.HyperShiftLogForwarder, clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {

	if len(clf.Spec.Inputs) < 1 {
		clf.Spec.Inputs = append(clf.Spec.Inputs, InputHTTPServerSpec)
	}

	//if len(hlf.Spec.Inputs) > 0 {
	//	for _, input := range hlf.Spec.Inputs {
	//		if !strings.Contains(input.Name, constants.CustomerManagedRuleNamePrefix) {
	//			input.Name = constants.CustomerManagedRuleNamePrefix + "-" + input.Name
	//		}
	//		clf.Spec.Inputs = append(clf.Spec.Inputs, input)
	//	}
	//}
	return clf
}

// BuildOutputsFromHLF builds the CLF outputs from the HLF
func BuildOutputsFromHLF(hlf *v1alpha1.HyperShiftLogForwarder, clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {
	if len(hlf.Spec.Outputs) > 0 {
		for _, output := range hlf.Spec.Outputs {
			if !strings.Contains(output.Name, constants.CustomerManagedRuleNamePrefix) {
				output.Name = constants.CustomerManagedRuleNamePrefix + "-" + output.Name
			}
			clf.Spec.Outputs = append(clf.Spec.Outputs, output)
		}
	}

	return clf
}

// BuildPipelinesFromHLF builds the CLF pipelines from the HLF
func BuildPipelinesFromHLF(hlf *v1alpha1.HyperShiftLogForwarder, clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {
	if len(hlf.Spec.Pipelines) > 0 {
		autoGenName := "auto-generated-name-"
		for i, ppl := range hlf.Spec.Pipelines {
			if ppl.Name == "" {
				ppl.Name = autoGenName + strconv.Itoa(i)
			}
			if !strings.Contains(ppl.Name, constants.CustomerManagedRuleNamePrefix) {
				ppl.Name = constants.CustomerManagedRuleNamePrefix + "-" + ppl.Name
			}
			clf.Spec.Pipelines = append(clf.Spec.Pipelines, ppl)
		}
	}

	return clf
}

// BuildFiltersFromHLF builds the CLF filters from the HLF
func BuildFiltersFromHLF(hlf *v1alpha1.HyperShiftLogForwarder, clf *loggingv1.ClusterLogForwarder) *loggingv1.ClusterLogForwarder {
	if len(hlf.Spec.Filters) > 0 {
		for _, f := range hlf.Spec.Filters {
			if !strings.Contains(f.Name, constants.CustomerManagedRuleNamePrefix) {
				f.Name = constants.CustomerManagedRuleNamePrefix + "-" + f.Name
			}
			clf.Spec.Filters = append(clf.Spec.Filters, f)
		}
	}
	return clf
}

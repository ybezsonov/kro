package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker"
	"github.com/google/cel-go/ext"
	"github.com/google/cel-go/interpreter"
	k8s "k8s.io/apiserver/pkg/cel/library"
)

var celEnvOptions = []cel.EnvOption{
	// 1.0 (1.23)
	cel.HomogeneousAggregateLiterals(),
	cel.EagerlyValidateDeclarations(true),
	cel.DefaultUTCTimeZone(true),
	k8s.URLs(),
	k8s.Regex(),
	k8s.Lists(),

	// 1.27
	// k8s.Authz(),

	// 1.28
	cel.CrossTypeNumericComparisons(true),
	cel.OptionalTypes(),
	k8s.Quantity(),

	// 1.29 (see also validator.ExtendedValidations())
	cel.ASTValidators(
		cel.ValidateDurationLiterals(),
		cel.ValidateTimestampLiterals(),
		cel.ValidateRegexLiterals(),
		cel.ValidateHomogeneousAggregateLiterals(),
	),

	// Strings (from 1.29 onwards)
	ext.Strings(ext.StringsVersion(2)),
	// Set library (1.29 onwards)
	ext.Sets(),

	// cel-go v0.17.7 introduced CostEstimatorOptions.
	// Previous the presence has a cost of 0 but cel fixed it to 1. We still set to 0 here to avoid breaking changes.
	cel.CostEstimatorOptions(checker.PresenceTestHasCost(false)),
}

var celProgramOptions = []cel.ProgramOption{
	cel.EvalOptions(cel.OptOptimize, cel.OptTrackCost),

	// cel-go v0.17.7 introduced CostTrackerOptions.
	// Previous the presence has a cost of 0 but cel fixed it to 1. We still set to 0 here to avoid breaking changes.
	cel.CostTrackerOptions(interpreter.PresenceTestHasCost(false)),
}

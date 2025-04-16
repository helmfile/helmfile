package config

import "github.com/helmfile/helmfile/pkg/common"

// ShouldIncludeCRDs determines if CRDs should be included in the operation.
// It returns true only when:
//   - includeCRDs flag is explicitly provided on the command line and set to true
//   - AND skipCRDs flag is not provided on the command line
//
// This ensures that CRDs are only included when explicitly requested and not
// contradicted by the skipCRDs flag.
func ShouldIncludeCRDs(includeCRDsFlag, skipCRDsFlag common.BoolFlag) bool {
	includeCRDsExplicit := includeCRDsFlag.WasExplicitlySet() && includeCRDsFlag.Value()
	skipCRDsProvided := skipCRDsFlag.WasExplicitlySet()

	return includeCRDsExplicit && !skipCRDsProvided
}

// Copyright 2025 The Kube Resource Orchestrator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parser

import (
	"fmt"
	"strings"
)

// This function parses resource condition expressions.
// These expressions need to be standalone expressions
// so, this function also does some validation.
// At the end we return the expressions with '${}' removed
//
// To be honest I wouldn't necessarily call it parse, since
// we are mostly just validating, without caring what's in
// the expression. Maybe we can rename it in the future 🤔
func ParseConditionExpressions(conditions []string) ([]string, error) {
	expressions := make([]string, 0, len(conditions))

	for _, e := range conditions {
		ok, err := isStandaloneExpression(e)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("only standalone expressions are allowed")
		}
		expressions = append(expressions, strings.Trim(e, "${}"))
	}

	return expressions, nil
}

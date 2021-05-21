// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
)

var (
	//go:embed templates/main.tpl.tf
	mainFile string
	//go:embed templates/terraform.tfvars
	terraformTFVars []byte
	//go:embed templates/variables.tf
	variablesTF string

	mainTemplate *template.Template
)

func init() {
	var err error
	mainTemplate, err = template.
		New("main.tf").
		Funcs(sprig.TxtFuncMap()).
		Funcs(map[string]interface{}{
			"dnsServers": dnsServers,
		}).Parse(mainFile)

	if err != nil {
		panic(err)
	}
}

// renders the list of dnsServers as a string
func dnsServers(servers []string) string {
	result := ""
	for _, server := range servers {
		result = fmt.Sprintf("%s%q, ", result, server)
	}
	return strings.TrimSuffix(result, ", ")
}

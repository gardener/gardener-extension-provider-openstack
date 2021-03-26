#!/bin/bash
#
# Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

mkdir -p "$(dirname $0)/../.github" "$(dirname $0)/../.github/ISSUE_TEMPLATE"

for file in `find "$(dirname $0)"/../vendor/github.com/gardener/gardener/.github -name '*.md'`; do
  cat "$file" |\
    sed 's/operating Gardener/working with this Gardener extension/g' |\
    sed 's/to the Gardener project/for this extension/g' |\
    sed 's/to Gardener/to this extension/g' |\
    sed 's/- Gardener version:/- Gardener version (if relevant):\n- Extension version:/g' |\
    sed 's/\/priority 3/\/priority 3\n\/platform openstack/g' |\
    sed 's/\/kind TODO/\/kind TODO\n\/platform openstack/g' \
  > "$(dirname $0)/../.github/${file#*.github/}"
done

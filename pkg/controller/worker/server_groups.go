// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gardener/gardener/pkg/utils"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servergroups"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

func isServerGroupRequired(config *api.WorkerConfig) bool {
	return config != nil && config.ServerGroup != nil && config.ServerGroup.Policy != ""
}

func generateServerGroupName(clusterName, poolName string, sgCfg api.ServerGroup) string {
	// Policy cannot be empty here as we check for it in isServerGroupRequired before calling this function
	data := fmt.Sprintf("policy=%s", sgCfg.Policy)
	hash := utils.ComputeSHA256Hex([]byte(data))[:5]

	return fmt.Sprintf("%s-%s-%s", clusterName, poolName, hash)
}

func filterServerGroupsByPrefix(sgs []servergroups.ServerGroup, prefix string) []servergroups.ServerGroup {
	var result []servergroups.ServerGroup
	for _, sg := range sgs {
		if strings.HasPrefix(sg.Name, prefix) {
			result = append(result, sg)
		}
	}

	return result
}

// serverGroupDependencySet is a set implementation for ServerGroupDependency objects that uses the PoolName as identifying key.
type serverGroupDependencySet struct {
	set map[string]api.ServerGroupDependency
}

// newServerGroupDependencySet creates a new serverGroupDependencySet.
func newServerGroupDependencySet(deps []api.ServerGroupDependency) serverGroupDependencySet {
	m := make(map[string]api.ServerGroupDependency, len(deps))
	for _, d := range deps {
		m[d.PoolName] = d
	}

	return serverGroupDependencySet{m}
}

// upsert inserts a new value or updates a value present in the set.
func (s *serverGroupDependencySet) upsert(d *api.ServerGroupDependency) {
	if d == nil {
		return
	}
	s.set[d.PoolName] = *d
}

// getByPoolName retrieves a ServerGroupDependency if it matches the provided PoolName. It returns nil if there is no matching entry in the set.
func (s *serverGroupDependencySet) getByPoolName(pn string) *api.ServerGroupDependency {
	d, ok := s.set[pn]
	if !ok {
		return nil
	}
	return &d
}

// getById retrieves a ServerGroupDependency if it matches the provided ID. It returns nil if there is no matching entry in the set.
func (s *serverGroupDependencySet) getById(id string) *api.ServerGroupDependency {
	for _, v := range s.set {
		if v.ID == id {
			return &v
		}
	}

	return nil
}

// deleteByPoolName deletes a ServerGroupDependency if it matches the provided PoolName. It is a no-op if there is no matching entry in the set.
func (s *serverGroupDependencySet) deleteByPoolName(pn string) {
	delete(s.set, pn)
}

// extract produces a slice from the elements contained in the set, sorted by PoolName.
func (s *serverGroupDependencySet) extract() []api.ServerGroupDependency {
	if len(s.set) == 0 {
		return nil
	}

	r := make([]api.ServerGroupDependency, 0, len(s.set))
	for _, v := range s.set {
		r = append(r, v)
	}

	// sort resulting slice to avoid randomization from map
	sort.Slice(r, func(i, j int) bool {
		return r[i].PoolName < r[j].PoolName
	})
	return r
}

// forEach executes function f for all elements contained in the set. If an error occurs the execution stops immediately.
func (s *serverGroupDependencySet) forEach(f func(dependency api.ServerGroupDependency) error) error {
	for _, v := range s.set {
		if err := f(v); err != nil {
			return err
		}
	}
	return nil
}

/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package devicewatcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopologyChanged(t *testing.T) {
	tests := []struct {
		name     string
		oldUUIDs []string
		newUUIDs []string
		want     bool
	}{
		{
			name:     "Same UUIDs",
			oldUUIDs: []string{"GPU-1", "GPU-2"},
			newUUIDs: []string{"GPU-1", "GPU-2"},
			want:     false,
		},
		{
			name:     "Same UUIDs Different Order",
			oldUUIDs: []string{"GPU-1", "GPU-2"},
			newUUIDs: []string{"GPU-2", "GPU-1"},
			want:     false,
		},
		{
			name:     "Different Count",
			oldUUIDs: []string{"GPU-1", "GPU-2"},
			newUUIDs: []string{"GPU-1"},
			want:     true,
		},
		{
			name:     "Different UUIDs Same Count",
			oldUUIDs: []string{"GPU-1", "GPU-2"},
			newUUIDs: []string{"GPU-1", "GPU-3"},
			want:     true,
		},
		{
			name:     "Empty to Non-Empty",
			oldUUIDs: []string{},
			newUUIDs: []string{"GPU-1"},
			want:     true,
		},
		{
			name:     "Non-Empty to Empty",
			oldUUIDs: []string{"GPU-1"},
			newUUIDs: []string{},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := topologyChanged(tt.oldUUIDs, tt.newUUIDs)
			assert.Equal(t, tt.want, got)
		})
	}
}

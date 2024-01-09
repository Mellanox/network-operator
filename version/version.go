/*
Copyright 2020 NVIDIA

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package version holds information about the version of the Operator.
package version

// This version information is added during the build process and passed to go using `ldflags`
// See: https://github.com/Mellanox/network-operator/blob/60efcfe699285a20dd9840383f63fd64aa9a40f6/Makefile#L28-L32
var (
	// Version is a semver version number.
	Version = "x.x.x"
	// Date is the date of the release.
	Date = "1970-01-01T00:00:00"
	// Commit is the release commit.
	Commit = "N/A"
)

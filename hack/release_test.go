/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

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

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
)

func releaseImage(repository, image, version string) *ReleaseImageSpec {
	return &ReleaseImageSpec{ImageSpec: mellanoxv1alpha1.ImageSpec{
		Repository: repository,
		Image:      image,
		Version:    version,
	}}
}

var _ = Describe("Release image digest resolution", func() {
	const (
		targetRepository   = "nvcr.io/nvidia/mellanox"
		sourceRepository   = "nvcr.io/nvstaging/mellanox"
		externalRepository = "nvcr.io/nvidia/doca"
	)

	It("looks up managed and driver images in staging while rendering production references", func() {
		targetRelease := Release{
			NetworkOperator: releaseImage(targetRepository, "network-operator", "v26.4.1"),
			Mofed: releaseImage(targetRepository, "doca-driver",
				"doca3.5.0-26.07-0.0.9.0-0"),
			DOCATelemetryService: releaseImage(externalRepository, "doca_telemetry",
				"1.25.5-doca3.4.0-host"),
		}
		digestSourceRelease := Release{
			NetworkOperator: releaseImage(sourceRepository, "network-operator", "v26.4.1"),
			Mofed: releaseImage(sourceRepository, "doca-driver",
				"doca3.5.0-26.07-0.0.9.0-0"),
			DOCATelemetryService: releaseImage(externalRepository, "doca_telemetry",
				"1.25.5-doca3.4.0-host"),
		}

		lookupRepositories := map[string]string{}
		resolveImage := func(repo, image, _ string) (string, error) {
			lookupRepositories[image] = repo
			return "sha256:" + image, nil
		}
		resolveDocaDrivers := func(repo, image, _ string) ([]string, error) {
			lookupRepositories[image] = repo
			return []string{"sha256:driver-a", "sha256:driver-b"}, nil
		}

		Expect(resolveImagesShaWithResolvers(&targetRelease, &digestSourceRelease,
			resolveImage, resolveDocaDrivers)).To(Succeed())

		Expect(lookupRepositories).To(HaveKeyWithValue("network-operator", sourceRepository))
		Expect(lookupRepositories).To(HaveKeyWithValue("doca-driver", sourceRepository))
		Expect(lookupRepositories).To(HaveKeyWithValue("doca_telemetry", externalRepository))
		Expect(targetRelease.NetworkOperator.Shas[0].ImageRef).To(Equal(
			targetRepository + "/network-operator@sha256:network-operator"))
		Expect(targetRelease.Mofed.Shas).To(HaveLen(2))
		Expect(targetRelease.Mofed.Shas[0].ImageRef).To(Equal(
			targetRepository + "/doca-driver@sha256:driver-a"))
		Expect(targetRelease.Mofed.Shas[1].ImageRef).To(Equal(
			targetRepository + "/doca-driver@sha256:driver-b"))
		Expect(targetRelease.DOCATelemetryService.Shas[0].ImageRef).To(Equal(
			externalRepository + "/doca_telemetry@sha256:doca_telemetry"))
	})

	It("uses the target release repository when no digest source release is provided", func() {
		targetRelease := Release{
			NetworkOperator: releaseImage(targetRepository, "network-operator", "v26.4.1"),
		}
		var lookupRepository string
		resolveImage := func(repo, _, _ string) (string, error) {
			lookupRepository = repo
			return "sha256:operator", nil
		}

		Expect(resolveImagesShaWithResolvers(&targetRelease, nil, resolveImage, nil)).To(Succeed())

		Expect(lookupRepository).To(Equal(targetRepository))
		Expect(targetRelease.NetworkOperator.Shas[0].ImageRef).To(Equal(
			targetRepository + "/network-operator@sha256:operator"))
	})

	DescribeTable("rejects an inconsistent digest source release",
		func(source *Release, expectedError string) {
			target := Release{
				NetworkOperator: releaseImage(targetRepository, "network-operator", "v26.4.1"),
			}
			Expect(validateDigestSourceRelease(&target, source)).To(MatchError(ContainSubstring(expectedError)))
		},
		Entry("with a missing field", &Release{}, "missing NetworkOperator"),
		Entry("with an empty source repository", &Release{
			NetworkOperator: releaseImage("", "network-operator", "v26.4.1"),
		}, "NetworkOperator repository is empty"),
		Entry("with a different image", &Release{
			NetworkOperator: releaseImage(sourceRepository, "other-operator", "v26.4.1"),
		}, "NetworkOperator image"),
		Entry("with a different version", &Release{
			NetworkOperator: releaseImage(sourceRepository, "network-operator", "v26.4.2"),
		}, "NetworkOperator version"),
	)
})

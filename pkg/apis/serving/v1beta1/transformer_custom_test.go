/*
Copyright 2021 The KServe Authors.

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

package v1beta1

import (
	"fmt"
	"testing"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kserve/kserve/pkg/constants"
)

func TestTransformerDefaulter(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	defaultResource := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("2Gi"),
	}
	config := &InferenceServicesConfig{
		Resource: ResourceConfig{
			CPULimit:      "1",
			MemoryLimit:   "2Gi",
			CPURequest:    "1",
			MemoryRequest: "2Gi",
		},
	}
	scenarios := map[string]struct {
		spec     TransformerSpec
		expected TransformerSpec
	}{
		"DefaultResources": {
			spec: TransformerSpec{
				PodSpec: PodSpec{
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name:  "STORAGE_URI",
									Value: "hdfs://modelzoo",
								},
							},
						},
					},
				},
			},
			expected: TransformerSpec{
				PodSpec: PodSpec{
					Containers: []corev1.Container{
						{
							Name: constants.InferenceServiceContainerName,
							Env: []corev1.EnvVar{
								{
									Name:  "STORAGE_URI",
									Value: "hdfs://modelzoo",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: defaultResource,
								Limits:   defaultResource,
							},
						},
					},
				},
			},
		},
	}

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			CustomTransformer := NewCustomTransformer(&scenario.spec.PodSpec)
			CustomTransformer.Default(config)
			if !g.Expect(scenario.spec).To(gomega.Equal(scenario.expected)) {
				t.Errorf("got %v, want %v", scenario.spec, scenario.expected)
			}
		})
	}
}

func TestCreateTransformerContainer(t *testing.T) {
	requestedResource := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"cpu": resource.Quantity{
				Format: "100",
			},
			"memory": resource.MustParse("1Gi"),
		},
		Requests: corev1.ResourceList{
			"cpu": resource.Quantity{
				Format: "90",
			},
			"memory": resource.MustParse("1Gi"),
		},
	}
	g := gomega.NewGomegaWithT(t)
	scenarios := map[string]struct {
		isvc                  InferenceService
		expectedContainerSpec *corev1.Container
	}{
		"ContainerSpecWithCustomImage": {
			isvc: InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sklearn",
				},
				Spec: InferenceServiceSpec{
					Predictor: PredictorSpec{
						SKLearn: &SKLearnSpec{
							PredictorExtensionSpec: PredictorExtensionSpec{
								StorageURI: proto.String("gs://someUri"),
								Container: corev1.Container{
									Image:     "customImage:0.1.0",
									Resources: requestedResource,
								},
							},
						},
					},
					Transformer: &TransformerSpec{
						PodSpec: PodSpec{
							Containers: []corev1.Container{
								{
									Image: "transformer:0.1.0",
									Env: []corev1.EnvVar{
										{
											Name:  "STORAGE_URI",
											Value: "hdfs://modelzoo",
										},
									},
									Resources: requestedResource,
								},
							},
						},
					},
				},
			},
			expectedContainerSpec: &corev1.Container{
				Image:     "transformer:0.1.0",
				Name:      constants.InferenceServiceContainerName,
				Resources: requestedResource,
				Args: []string{
					"--model_name",
					"someName",
					"--predictor_host",
					fmt.Sprintf("%s.%s", constants.DefaultPredictorServiceName("someName"), "default"),
					"--http_port",
					"8080",
				},
				Env: []corev1.EnvVar{
					{
						Name:  "STORAGE_URI",
						Value: "hdfs://modelzoo",
					},
				},
			},
		},
		"ContainerSpecWithContainerConcurrency": {
			isvc: InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sklearn",
				},
				Spec: InferenceServiceSpec{
					Predictor: PredictorSpec{
						ComponentExtensionSpec: ComponentExtensionSpec{
							ContainerConcurrency: proto.Int64(1),
						},
						SKLearn: &SKLearnSpec{
							PredictorExtensionSpec: PredictorExtensionSpec{
								StorageURI: proto.String("gs://someUri"),
								Container: corev1.Container{
									Resources: requestedResource,
								},
							},
						},
					},
					Transformer: &TransformerSpec{
						ComponentExtensionSpec: ComponentExtensionSpec{
							ContainerConcurrency: proto.Int64(2),
						},
						PodSpec: PodSpec{
							Containers: []corev1.Container{
								{
									Image: "transformer:0.1.0",
									Env: []corev1.EnvVar{
										{
											Name:  "STORAGE_URI",
											Value: "hdfs://modelzoo",
										},
									},
									Resources: requestedResource,
								},
							},
						},
					},
				},
			},
			expectedContainerSpec: &corev1.Container{
				Image:     "transformer:0.1.0",
				Name:      constants.InferenceServiceContainerName,
				Resources: requestedResource,
				Args: []string{
					"--model_name",
					"someName",
					"--predictor_host",
					fmt.Sprintf("%s.%s", constants.DefaultPredictorServiceName("someName"), "default"),
					"--http_port",
					"8080",
					"--workers",
					"2",
				},
				Env: []corev1.EnvVar{
					{
						Name:  "STORAGE_URI",
						Value: "hdfs://modelzoo",
					},
				},
			},
		},
		"ContainerSpecWithWorker": {
			isvc: InferenceService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sklearn",
				},
				Spec: InferenceServiceSpec{
					Predictor: PredictorSpec{
						ComponentExtensionSpec: ComponentExtensionSpec{
							ContainerConcurrency: proto.Int64(4),
						},
						SKLearn: &SKLearnSpec{
							PredictorExtensionSpec: PredictorExtensionSpec{
								StorageURI: proto.String("gs://someUri"),
								Container: corev1.Container{
									Resources: requestedResource,
									Args: []string{
										"--workers",
										"1",
									},
								},
							},
						},
					},
					Transformer: &TransformerSpec{
						ComponentExtensionSpec: ComponentExtensionSpec{
							ContainerConcurrency: proto.Int64(2),
						},
						PodSpec: PodSpec{
							Containers: []corev1.Container{
								{
									Image: "transformer:0.1.0",
									Env: []corev1.EnvVar{
										{
											Name:  "STORAGE_URI",
											Value: "hdfs://modelzoo",
										},
									},
									Resources: requestedResource,
									Args: []string{
										"--model_name",
										"someName",
										"--predictor_host",
										"localhost",
										"--http_port",
										"8080",
										"--workers",
										"1",
									},
								},
							},
						},
					},
				},
			},
			expectedContainerSpec: &corev1.Container{
				Image:     "transformer:0.1.0",
				Name:      constants.InferenceServiceContainerName,
				Resources: requestedResource,
				Args: []string{
					"--model_name",
					"someName",
					"--predictor_host",
					"localhost",
					"--http_port",
					"8080",
					"--workers",
					"1",
				},
				Env: []corev1.EnvVar{
					{
						Name:  "STORAGE_URI",
						Value: "hdfs://modelzoo",
					},
				},
			},
		},
	}
	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			transformer := scenario.isvc.Spec.Transformer.GetImplementation()
			transformer.Default(nil)
			res := transformer.GetContainer(metav1.ObjectMeta{Name: "someName", Namespace: "default"}, &scenario.isvc.Spec.Transformer.ComponentExtensionSpec,
				nil, constants.DefaultPredictorServiceName("someName"))
			if !g.Expect(res).To(gomega.Equal(scenario.expectedContainerSpec)) {
				t.Errorf("got %q, want %q", res, scenario.expectedContainerSpec)
			}
		})
	}
}

func TestTransformerGetProtocol(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	scenarios := map[string]struct {
		spec    *CustomTransformer
		matcher types.GomegaMatcher
	}{
		"DefaultProtocol": {
			spec: &CustomTransformer{
				PodSpec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "transformer:0.1.0",
							Env: []corev1.EnvVar{
								{
									Name:  "STORAGE_URI",
									Value: "hdfs://modelzoo",
								},
							},
							Args: []string{
								"--model_name",
								"someName",
								"--predictor_host",
								"localhost",
								"--http_port",
								"8080",
								"--workers",
								"1",
							},
						},
					},
				},
			},

			matcher: gomega.Equal(constants.ProtocolV1),
		},
		"ProtocolSpecified": {
			spec: &CustomTransformer{
				PodSpec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "transformer:0.1.0",
							Env: []corev1.EnvVar{
								{
									Name:  "STORAGE_URI",
									Value: "hdfs://modelzoo",
								},
								{
									Name:  constants.CustomSpecProtocolEnvVarKey,
									Value: string(constants.ProtocolV2),
								},
							},
							Args: []string{
								"--model_name",
								"someName",
								"--predictor_host",
								"localhost",
								"--http_port",
								"8080",
								"--workers",
								"1",
							},
						},
					},
				},
			},
			matcher: gomega.Equal(constants.ProtocolV2),
		},
	}
	for _, scenario := range scenarios {
		protocol := scenario.spec.GetProtocol()
		g.Expect(protocol).To(scenario.matcher)
	}
}

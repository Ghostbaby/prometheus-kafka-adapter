// Copyright 2018 Telefónica
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

package main

import (
	"encoding/json"
	"io/ioutil"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	promcli "github.com/ghostbaby/prometheus-kafka-adapter/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/linkedin/goavro"
	"fmt"
	"net/http"
	"bytes"
	"strconv"
	"time"
	"math"
)

// Serializer represents an abstract metrics serializer
type Serializer interface {
	Marshal(metric map[string]interface{}) ([]byte, error)
}

type PodInfo struct {
	Pod_IP	string `json:"pod_ip"`
	Pod_Name	string	`json:"pod_name"`
}

func GetPodIP(np string, name string,k8swatch string) (error, string) {
	res, err := http.Post(k8swatch,"",bytes.NewBuffer([]byte(name)))
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
	}

	defer res.Body.Close()

	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
	}
	//fmt.Printf(string(content))
	var podInfo PodInfo
	if err := json.Unmarshal(content, &podInfo); err == nil {
		return nil,podInfo.Pod_IP
	}else {
		return err,""

	}
}

// Serialize generates the JSON representation for a given Prometheus metric.
func Serialize(s Serializer, req *prompb.WriteRequest,k8swatch string, promeURL string, nameSpace string) ([][]byte, error) {
	result := [][]byte{}

	for _, ts := range req.Timeseries {

		labels := make(map[string]string, len(ts.Labels))

		for _, l := range ts.Labels {
			labels[string(model.LabelName(l.Name))] = string(model.LabelValue(l.Value))
		}


		for _, sample := range ts.Samples {
			metricsName := string(labels["__name__"])
			metricsNamespace := string(labels["namespace"])
			metricsContainerName := string(labels["container_name"])
			//if strings.Contains(metricsName, "container") &&
			if metricsName == "container_cpu_usage_seconds_total" &&
				metricsNamespace == nameSpace &&
				metricsContainerName != "POD"{

				//epoch := time.Unix(sample.Timestamp/1000, 0).Unix()
				metricsService := string(labels["service"])
				var endpoint string
				if metricsService == "kubelet" {
					endpoint = string(labels["pod_name"])
				}else if metricsService == "kube-state-metrics" {
					endpoint = string(labels["pod"])
				}

				err,podIP := GetPodIP(metricsNamespace,endpoint,k8swatch)
				if err != nil {
					fmt.Println(err)
					fmt.Println(labels)
					return nil,err
				}

				timestamp,value,err := promcli.GetPromContainerCpuUsage(endpoint,promeURL,sample.Timestamp)
				if err != nil {
					return nil,err
				}

				reqcpuName := string(labels["container_name"]) + "_req_cpu"

				err,reqCPU := GetPodIP(metricsNamespace,reqcpuName,k8swatch)
				if err != nil {
					return nil,err
				}

				reqCpuFlat64,err := strconv.ParseFloat(reqCPU,64)
				if err != nil {
					return nil,err
				}

				cpuPer := value / reqCpuFlat64


				m := map[string]interface{}{
					"timestamp": timestamp,
					"value": Decimal(cpuPer * 100),
					"metric":      metricsName,
					"endpoint":	endpoint,
					"ip": podIP,
					"tags":    labels,
					"counterType": "GAUGE",
					"application": "docker",
					"step": 30,
				}

				data, err := s.Marshal(m)
				if err != nil {
					logrus.WithError(err).Errorln("couldn't marshal timeseries.")
				}

				result = append(result, data)
			}else if metricsName == "container_memory_usage_bytes" &&
				metricsNamespace == nameSpace &&
				metricsContainerName != "POD" {

					metricsService := string(labels["service"])
					var endpoint string
					if metricsService == "kubelet" {
						endpoint = string(labels["pod_name"])
					}else if metricsService == "kube-state-metrics" {
						endpoint = string(labels["pod"])
					}

					err,podIP := GetPodIP(metricsNamespace,endpoint,k8swatch)
					if err != nil {
						fmt.Println(err)
						fmt.Println(labels)
						return nil,err
					}

				reqmemName := string(labels["container_name"]) + "_req_mem"

				err,reqMEM := GetPodIP(metricsNamespace,reqmemName,k8swatch)
				if err != nil {
					return nil,err
				}

				reqMemFlat64,err := strconv.ParseFloat(reqMEM,64)
				if err != nil {
					return nil,err
				}

				memPer := sample.Value / reqMemFlat64

				m := map[string]interface{}{
					//"timestamp": epoch.Format(time.RFC3339),
					"timestamp": time.Unix(sample.Timestamp/1000, 0).Unix(),
					"value": Decimal(memPer * 100),
					"metric":      metricsName,
					"endpoint":	endpoint,
					"ip": podIP,
					"tags":    labels,
					"counterType": "GAUGE",
					"application": "docker",
					"step": 30,
				}

				data, err := s.Marshal(m)
				if err != nil {
					logrus.WithError(err).Errorln("couldn't marshal timeseries.")
				}

				result = append(result, data)


			}else if metricsName == "container_network_receive_bytes_total" &&
				metricsNamespace == nameSpace {

				metricsService := string(labels["service"])
				var endpoint string
				if metricsService == "kubelet" {
					endpoint = string(labels["pod_name"])
				}else if metricsService == "kube-state-metrics" {
					endpoint = string(labels["pod"])
				}

				err,podIP := GetPodIP(metricsNamespace,endpoint,k8swatch)
				if err != nil {
					fmt.Println(err)
					fmt.Println(labels)
					return nil,err
				}

				timestamp,value,err := promcli.GetPromContainerNetworkUsage(endpoint,promeURL,sample.Timestamp,"container_network_receive_bytes_total")
				if err != nil {
					return nil,err
				}

				m := map[string]interface{}{
					//"timestamp": epoch.Format(time.RFC3339),
					"timestamp": timestamp,
					"value": value,
					"metric":      metricsName,
					"endpoint":	endpoint,
					"ip": podIP,
					"tags":    labels,
					"counterType": "GAUGE",
					"application": "docker",
					"step": 30,
				}

				data, err := s.Marshal(m)
				if err != nil {
					logrus.WithError(err).Errorln("couldn't marshal timeseries.")
				}

				result = append(result, data)
			}else if metricsName == "container_network_transmit_bytes_total" &&
				metricsNamespace == nameSpace {

				metricsService := string(labels["service"])
				var endpoint string
				if metricsService == "kubelet" {
					endpoint = string(labels["pod_name"])
				}else if metricsService == "kube-state-metrics" {
					endpoint = string(labels["pod"])
				}

				err,podIP := GetPodIP(metricsNamespace,endpoint,k8swatch)
				if err != nil {
					fmt.Println(err)
					fmt.Println(labels)
					return nil,err
				}

				timestamp,value,err := promcli.GetPromContainerNetworkUsage(endpoint,promeURL,sample.Timestamp,"container_network_transmit_bytes_total")
				if err != nil {
					return nil,err
				}

				m := map[string]interface{}{
					//"timestamp": epoch.Format(time.RFC3339),
					"timestamp": timestamp,
					"value": value,
					"metric":      metricsName,
					"endpoint":	endpoint,
					"ip": podIP,
					"tags":    labels,
					"counterType": "GAUGE",
					"application": "docker",
					"step": 30,
				}

				data, err := s.Marshal(m)
				if err != nil {
					logrus.WithError(err).Errorln("couldn't marshal timeseries.")
				}

				result = append(result, data)
			}
		}
	}

	return result, nil
}

// JSONSerializer represents a metrics serializer that writes JSON
type JSONSerializer struct {
}

func (s *JSONSerializer) Marshal(metric map[string]interface{}) ([]byte, error) {
	return json.Marshal(metric)
}

func NewJSONSerializer() (*JSONSerializer, error) {
	return &JSONSerializer{}, nil
}

// AvroJSONSerializer represents a metrics serializer that writes Avro-JSON
type AvroJSONSerializer struct {
	codec *goavro.Codec
}

func (s *AvroJSONSerializer) Marshal(metric map[string]interface{}) ([]byte, error) {
	return s.codec.TextualFromNative(nil, metric)
}

// NewAvroJSONSerializer builds a new instance of the AvroJSONSerializer
func NewAvroJSONSerializer(schemaPath string) (*AvroJSONSerializer, error) {
	schema, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		logrus.WithError(err).Errorln("couggldn't read avro schema")
		return nil, err
	}

	codec, err := goavro.NewCodec(string(schema))
	if err != nil {
		logrus.WithError(err).Errorln("couldn't create avro codec")
		return nil, err
	}

	return &AvroJSONSerializer{
		codec: codec,
	}, nil
}

func Decimal(value float64) float64 {
	return math.Trunc(value*1e2+0.5) * 1e-2
}
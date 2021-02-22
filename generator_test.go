// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package generator

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

var expectedFluentBitConfig = `@INCLUDE /etc/head_file.conf

[INPUT]
    Name forward
    Tag tag
    Listen 127.0.0.1
    Port 24224

[INPUT]
    Name forward
    unix_path /var/run/fluentd.sock

@INCLUDE /etc/after_inputs.conf

[FILTER]
    Name   grep
    Match *
    Regex  log *failure*

[FILTER]
    Name   grep
    Match *
    Exclude log *success*

[FILTER]
    Name record_modifier
    Match *
    Record cluster default
    Record task_arn arn:aws:ecs:us-east-2:01234567891011:task/mycluster/3de392df-6bfa-470b-97ed-aa6f482cd7a6

@INCLUDE /etc/after_filters.conf

[OUTPUT]
    Name cloudwath
    Match *
    log_group_name my-group
    log_stream_prefix my-prefix-
    region us-west-2

[OUTPUT]
    Name firehose
    Match *
    delivery_stream my-stream
    region us-west-2

@INCLUDE /etc/end_file.conf
`

var expectedFluentDConfig = `@include /etc/head_file.conf

<source>
    @type forward
    tag tag
    Listen 127.0.0.1
    Port 24224
</source>

<source>
    @type forward
    unix_path /var/run/fluentd.sock
</source>

@include /etc/after_inputs.conf

<filter *>
    @type  grep
    <regexp>
        key log
        pattern *failure*
    </regexp>
</filter>

<filter *>
    @type  grep
    <exclude>
        key log
        pattern *success*
    </exclude>
</filter>

<filter *>
    @type record_transformer
    <record>
        cluster default
        task_arn arn:aws:ecs:us-east-2:01234567891011:task/mycluster/3de392df-6bfa-470b-97ed-aa6f482cd7a6
    </record>
</filter>

@include /etc/after_filters.conf

<match *>
    @type cloudwath
    log_group_name my-group
    log_stream_prefix my-prefix-
    region us-west-2
</match>

<match *>
    @type firehose
    delivery_stream my-stream
    region us-west-2
</match>

@include /etc/end_file.conf
`

func TestGenerateConfig(t *testing.T) {
	config := New()
	config.AddInput("forward", "tag", map[string]string{
		"Listen": "127.0.0.1",
		"Port":   "24224",
	}).AddInput("forward", "", map[string]string{
		"unix_path": "/var/run/fluentd.sock",
	}).AddIncludeFilter("*failure*", "log", "*").AddExcludeFilter("*success*", "log", "*")

	config.AddFieldToRecord("cluster", "default", "*").AddFieldToRecord("task_arn", "arn:aws:ecs:us-east-2:01234567891011:task/mycluster/3de392df-6bfa-470b-97ed-aa6f482cd7a6", "*")

	config.AddExternalConfig("/etc/head_file.conf", HeadOfFile)
	config.AddExternalConfig("/etc/after_inputs.conf", AfterInputs)
	config.AddExternalConfig("/etc/after_filters.conf", AfterFilters)
	config.AddExternalConfig("/etc/end_file.conf", EndOfFile)

	config.AddOutput("cloudwath", "*", map[string]string{
		"log_group_name":    "my-group",
		"region":            "us-west-2",
		"log_stream_prefix": "my-prefix-",
	}).AddOutput("firehose", "*", map[string]string{
		"delivery_stream": "my-stream",
		"region":          "us-west-2",
	})

	fluentbitConfig := new(bytes.Buffer)
	config.WriteFluentBitConfig(fluentbitConfig)
	assert.Equal(t, expectedFluentBitConfig, fluentbitConfig.String(), "Expected Fluent Bit Config to match")

	fluentDConfig := new(bytes.Buffer)
	config.WriteFluentdConfig(fluentDConfig)
	assert.Equal(t, expectedFluentDConfig, fluentDConfig.String(), "Expected FluentD Config to match")
}

func TestGenerateCustomTemplate(t *testing.T) {
	var customConfig = `
    <filter {{.Custom.myFilter.Tag}}>
        @type {{.Custom.myFilter.Name}}

        matchRegex /{{.Custom.myFilter.Regex}}/
        replacement '{{.Custom.myFilter.Replacement}}'
    </filter>

    <output {{.Custom.myOutput.Tag}}>
        @type {{.Custom.myOutput.Name}}
        {{ range $dest := .Custom.myOutput.Destinations }}
        <destination>
            host {{$dest.Host}}
            port {{$dest.Port}}
        </destination>
        {{- end }}
    </output>
    `
	var filterData = struct {
		Name        string
		Tag         string
		Regex       string
		Replacement string
	}{
		Name:        `myFilter`,
		Tag:         `foo`,
		Regex:       `foo`,
		Replacement: `bar`,
	}
	var outputData = struct {
		Name         string
		Tag          string
		Destinations []struct {
			Host, Port string
		}
	}{
		`myOutput`, `*`, []struct {
			Host string
			Port string
		}{
			{`0.0.0.0`, `9001`},
			{`127.0.0.1`, `9002`},
		},
	}
	var customData = map[string]interface{}{
		`myFilter`: filterData,
		`myOutput`: outputData,
	}
	var expectedConfig = `
    <filter foo>
        @type myFilter

        matchRegex /foo/
        replacement 'bar'
    </filter>

    <output *>
        @type myOutput
        
        <destination>
            host 0.0.0.0
            port 9001
        </destination>
        <destination>
            host 127.0.0.1
            port 9002
        </destination>
    </output>
    `

	config := New()
	config.AddCustom(customData)
	config.WithFluentdTemplate(customConfig)
	config.WithFluentBitTemplate(customConfig)

	fluentbitConfig := new(bytes.Buffer)
	config.WriteFluentBitConfig(fluentbitConfig)
	assert.Equal(t, expectedConfig, fluentbitConfig.String(), "Expected Fluent Bit Config to match")

	fluentDConfig := new(bytes.Buffer)
	config.WriteFluentdConfig(fluentDConfig)
	assert.Equal(t, expectedConfig, fluentDConfig.String(), "Expected FluentD Config to match")
}

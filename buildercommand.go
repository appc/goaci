// Copyright 2016 The appc Authors.
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
	"flag"
	"fmt"

	"github.com/appc/goaci/proj2aci"
)

// parameterMapper is an interface which should handle command line
// parameter handling specific to a proj2aci.BuilderCustomizations
// implementation.
type parameterMapper interface {
	Name() string
	SetupParameters(parameters *flag.FlagSet)
	GetBuilderCustomizations() proj2aci.BuilderCustomizations
}

// builderCommand is an implementation of command interface which
// mainly maps command line parameters to proj2aci.Builder's
// configuration and runs the builder.
type builderCommand struct {
	mapper parameterMapper
}

func newBuilderCommand(mapper parameterMapper) command {
	return &builderCommand{
		mapper: mapper,
	}
}

func (cmd *builderCommand) Name() string {
	custom := cmd.mapper.GetBuilderCustomizations()
	return custom.Name()
}

func (cmd *builderCommand) Run(name string, args []string) error {
	parameters := flag.NewFlagSet(name, flag.ExitOnError)
	cmd.mapper.SetupParameters(parameters)
	if err := parameters.Parse(args); err != nil {
		return err
	}
	if len(parameters.Args()) != 1 {
		return fmt.Errorf("Expected exactly one project to build, got %d", len(args))
	}
	custom := cmd.mapper.GetBuilderCustomizations()
	custom.GetCommonConfiguration().Project = parameters.Args()[0]
	builder := proj2aci.NewBuilder(custom)
	return builder.Run()
}

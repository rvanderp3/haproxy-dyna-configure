// Code generated by go generate; DO NOT EDIT.
/*
Copyright 2019 HAProxy Technologies

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
package parsers

import (
	"github.com/haproxytech/config-parser/common"
	"github.com/haproxytech/config-parser/errors"
	"github.com/haproxytech/config-parser/types"
)

func (p *DefaultServer) Init() {
	p.data = []types.DefaultServer{}
}

func (p *DefaultServer) GetParserName() string {
	return "default-server"
}

func (p *DefaultServer) Get(createIfNotExist bool) (common.ParserData, error) {
	if len(p.data) == 0 && !createIfNotExist {
		return nil, errors.ErrFetch
	}
	return p.data, nil
}

func (p *DefaultServer) GetOne(index int) (common.ParserData, error) {
	if index < 0 || index >= len(p.data) {
		return nil, errors.ErrFetch
	}
	return p.data[index], nil
}

func (p *DefaultServer) Delete(index int) error {
	if index < 0 || index >= len(p.data) {
		return errors.ErrFetch
	}
	copy(p.data[index:], p.data[index+1:])
	p.data[len(p.data)-1] = types.DefaultServer{}
	p.data = p.data[:len(p.data)-1]
	return nil
}

func (p *DefaultServer) Insert(data common.ParserData, index int) error {
	if data == nil {
		return errors.ErrInvalidData
	}
	switch newValue := data.(type) {
	case []types.DefaultServer:
		p.data = newValue
	case *types.DefaultServer:
		if index > -1 {
			if index > len(p.data) {
				return errors.ErrIndexOutOfRange
			}
			p.data = append(p.data, types.DefaultServer{})
			copy(p.data[index+1:], p.data[index:])
			p.data[index] = *newValue
		} else {
			p.data = append(p.data, *newValue)
		}
	case types.DefaultServer:
		if index > -1 {
			if index > len(p.data) {
				return errors.ErrIndexOutOfRange
			}
			p.data = append(p.data, types.DefaultServer{})
			copy(p.data[index+1:], p.data[index:])
			p.data[index] = newValue
		} else {
			p.data = append(p.data, newValue)
		}
	default:
		return errors.ErrInvalidData
	}
	return nil
}

func (p *DefaultServer) Set(data common.ParserData, index int) error {
	if data == nil {
		p.Init()
		return nil
	}
	switch newValue := data.(type) {
	case []types.DefaultServer:
		p.data = newValue
	case *types.DefaultServer:
		if index > -1 && index < len(p.data) {
			p.data[index] = *newValue
		} else if index == -1 {
			p.data = append(p.data, *newValue)
		} else {
			return errors.ErrIndexOutOfRange
		}
	case types.DefaultServer:
		if index > -1 && index < len(p.data) {
			p.data[index] = newValue
		} else if index == -1 {
			p.data = append(p.data, newValue)
		} else {
			return errors.ErrIndexOutOfRange
		}
	default:
		return errors.ErrInvalidData
	}
	return nil
}

func (p *DefaultServer) Parse(line string, parts, previousParts []string, comment string) (changeState string, err error) {
	if parts[0] == "default-server" {
		data, err := p.parse(line, parts, comment)
		if err != nil {
			return "", &errors.ParseError{Parser: "DefaultServer", Line: line}
		}
		p.data = append(p.data, *data)
		return "", nil
	}
	return "", &errors.ParseError{Parser: "DefaultServer", Line: line}
}

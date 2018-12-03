package pbjs

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

const tmpl = `// Generated by protoc-gen-twirp_typescript. DO NOT EDIT
import {{pbjsImport .Package}} from './{{.ImportPath}}.pb';
import {createTwirpAdapter} from 'pbjs-twirp';
import Axios from 'axios';

{{$twirpPrefix := .TwirpPrefix -}}

const getServiceMethodName = (fn: any): string => {
    {{- range $s := .Services}}
	{{- range $m := $s.Methods}}
	if (fn === {{$s.Package}}.{{$s.Name}}.prototype.{{lowerCamel $m}}) {
		return '{{$m}}';
    }
	{{- end}}
	{{- end}}

    throw new Error('Unknown Method');
};

{{range .Services}}
export const {{.Name}}PathPrefix = '{{$twirpPrefix}}/{{.Package}}.{{.Name}}/';

export const create{{.Name}} = (baseURL: string): {{.Package}}.{{.Name}} => {
	const axios = Axios.create({
        baseURL: baseURL + {{.Name}}PathPrefix,
        headers: {
          Accept: 'application/protobuf'
        }
    });

    return {{.Package}}.{{.Name}}.create(createTwirpAdapter(axios, getServiceMethodName));
};
{{- end}}
`

type service struct {
	Name    string
	Methods []string
	Package string
}

type tmplContext struct {
	Services    []service
	Package     string
	ImportPath  string
	TwirpPrefix string
}

func NewGenerator(twirpVersion string) *Generator {
	return &Generator{twirpVersion: twirpVersion}
}

type Generator struct {
	twirpVersion string
}

func (g *Generator) Generate(d *descriptor.FileDescriptorProto) ([]*plugin.CodeGeneratorResponse_File, error) {
	// skip WKT Timestamp, we don't do any special serialization for jsonpb.
	if *d.Name == "google/protobuf/timestamp.proto" {
		return []*plugin.CodeGeneratorResponse_File{}, nil
	}

	twirpPrefix := "/twirp"
	if g.twirpVersion == "v6" {
		twirpPrefix = ""
	}

	pkg := d.GetPackage()
	ctx := tmplContext{
		Package:     pkg,
		ImportPath:  baseName(d),
		TwirpPrefix: twirpPrefix,
	}

	for _, s := range d.Service {
		srv := service{
			Name:    s.GetName(),
			Methods: make([]string, 0),
			Package: pkg,
		}

		for _, m := range s.Method {
			srv.Methods = append(srv.Methods, *m.Name)
		}

		ctx.Services = append(ctx.Services, srv)
	}

	tmplFuncs := make(map[string]interface{})
	tmplFuncs["lowerCamel"] = lowerCamel
	tmplFuncs["pbjsImport"] = pbjsImport

	t, err := template.New("pbjs_client").Funcs(tmplFuncs).Parse(tmpl)
	if err != nil {
		return nil, err
	}

	b := bytes.NewBufferString("")
	err = t.Execute(b, ctx)
	if err != nil {
		return nil, err
	}

	cf := &plugin.CodeGeneratorResponse_File{}
	cf.Name = outFile(d)
	cf.Content = proto.String(b.String())

	return []*plugin.CodeGeneratorResponse_File{cf}, nil
}

func baseName(d *descriptor.FileDescriptorProto) string {
	n := filepath.Base(d.GetName())
	parts := strings.Split(n, ".")

	return parts[0]
}

func outFile(d *descriptor.FileDescriptorProto) *string {
	n := filepath.Base(d.GetName())
	parts := strings.Split(n, ".")

	return proto.String(parts[0] + ".twirp.ts")
}

func lowerCamel(s string) string {
	return strings.ToLower(s[0:1]) + s[1:]
}

func pbjsImport(packageName string) string {
	parts := strings.Split(packageName, ".")

	if len(parts) > 0 {
		return "{" + parts[0] + "}"
	}

	return ""
}

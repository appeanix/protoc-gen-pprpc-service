package main

import (
	"flag"
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

const (
	serviceProtoFileSuffix  = "services.proto"
	cmdArgDomainImportPath  = "domainPath"
	cmdArgUseCaseImportPath = "useCasePath"
	cmdArgDtsImportPath     = "dtsPath"
)

var (
	domainImportPath  *string
	useCaseImportPath *string
	dtsImportPath     *string
)

const transformTwirpErrFnTemplate = `
func transformTwirpError(err error) twirp.Error {
	twerr := twirp.NewError(twirp.Internal, err.Error())
	if domainErr, ok := err.(domain.Error); ok {
		twerr = twerr.WithMeta("domainCode", fmt.Sprintf("%d", domainErr.Code))
	}
	return twirp.WrapError(twerr, err)
}
`

func main() {

	var flags flag.FlagSet
	domainImportPath = flags.String(cmdArgDomainImportPath, "", "")
	useCaseImportPath = flags.String(cmdArgUseCaseImportPath, "", "")
	dtsImportPath = flags.String(cmdArgDtsImportPath, "", "")

	protogen.Options{ParamFunc: flags.Set}.Run(func(gen *protogen.Plugin) error {

		for _, f := range gen.Files {
			protoFileName := *f.Proto.Name
			if !f.Generate || !strings.HasSuffix(protoFileName, serviceProtoFileSuffix) {
				continue
			}

			goFileName := fmt.Sprintf("%s_pprpc.pb.go", strings.TrimRight(protoFileName, ".proto"))
			gf := gen.NewGeneratedFile(goFileName, f.GoImportPath)
			gf.P("// Code generated by protoc-gen-go-pprpc. DO NOT EDIT.")
			gf.P()
			gf.P("package ", f.GoPackageName)
			gf.Import("errors")
			gf.Import("github.com/twitchtv/twirp")
			gf.Import(cmdArgDomainImportPath)

			contextIdent := contextIdent(gf)
			dtsTransformIdent := dtsTransformIdent(gf)

			for _, srv := range f.Services {

				serviceRpcStruct := fmt.Sprintf("%sRpc", srv.GoName)
				useCaseIdent := useCaseIdent(gf, strings.Replace(srv.GoName, "Service", "UseCase", 1))

				gf.P("type ", serviceRpcStruct, " struct {")
				gf.P("UseCase ", useCaseIdent)
				gf.P("}")
				gf.P("")

				for _, method := range srv.Methods {
					var inName = method.Input.GoIdent.GoName
					var outName = method.Output.GoIdent.GoName

					gf.P("func (service ", serviceRpcStruct, ") ", method.GoName, "(_ ", contextIdent, ", pbIn *", inName, ") (*", outName, ", error)", "{")

					gf.P("var err error")
					gf.P("var ucIn ", requestPramIdent(gf, inName))
					gf.P("var ucOut *", responsePayloadIdent(gf, outName))
					gf.P("var pbOut ", outName)
					gf.P()

					gf.P("if err = ", dtsTransformIdent, "(pbIn, &ucIn); err != nil { ")
					gf.P("	return nil, err")
					gf.P("}")
					gf.P()

					gf.P("if ucOut, err = service.UseCase.", method.GoName, "(ucIn); err != nil {")
					gf.P("  return nil, transformTwirpError(err)")
					gf.P("}")
					gf.P()

					gf.P("if err = ", dtsTransformIdent, "(ucOut, &pbOut); err != nil { ")
					gf.P("	return nil, err")
					gf.P("}")
					gf.P()

					gf.P("return &pbOut, nil")

					gf.P("}")
				}
			}

			gf.P(transformTwirpErrFnTemplate)
		}

		return nil
	})
}

func domainIdent(gf *protogen.GeneratedFile, ident string) string {
	return gf.QualifiedGoIdent(protogen.GoIdent{
		GoName:       ident,
		GoImportPath: protogen.GoImportPath(*domainImportPath),
	})
}

func useCaseIdent(gf *protogen.GeneratedFile, ident string) string {
	return gf.QualifiedGoIdent(protogen.GoIdent{
		GoName:       ident,
		GoImportPath: protogen.GoImportPath(*useCaseImportPath),
	})
}

func contextIdent(gf *protogen.GeneratedFile) string {
	return gf.QualifiedGoIdent(protogen.GoIdent{
		GoName:       "Context",
		GoImportPath: "context",
	})
}

func dtsTransformIdent(gf *protogen.GeneratedFile) string {
	return gf.QualifiedGoIdent(protogen.GoIdent{
		GoName:       "Transform",
		GoImportPath: protogen.GoImportPath(*dtsImportPath),
	})
}

func requestPramIdent(gf *protogen.GeneratedFile, pbParamTypeName string) string {
	if strings.HasSuffix(pbParamTypeName, "Param") || strings.HasSuffix(pbParamTypeName, "Params") {
		return gf.QualifiedGoIdent(protogen.GoIdent{
			GoName:       pbParamTypeName,
			GoImportPath: protogen.GoImportPath(*useCaseImportPath),
		})
	}

	return gf.QualifiedGoIdent(protogen.GoIdent{
		GoName:       pbParamTypeName,
		GoImportPath: protogen.GoImportPath(*domainImportPath),
	})
}

func responsePayloadIdent(gf *protogen.GeneratedFile, respTypeName string) string {
	if strings.HasSuffix(respTypeName, "Response") {
		return gf.QualifiedGoIdent(protogen.GoIdent{
			GoName:       respTypeName,
			GoImportPath: protogen.GoImportPath(*useCaseImportPath),
		})
	}

	return gf.QualifiedGoIdent(protogen.GoIdent{
		GoName:       respTypeName,
		GoImportPath: protogen.GoImportPath(*domainImportPath),
	})
}

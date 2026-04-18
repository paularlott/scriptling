package extlibs

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/object"
)

// RegisterSecretLibrary registers the provider-agnostic secret access library.
func RegisterSecretLibrary(registrar interface{ RegisterLibrary(*object.Library) }, registry *secretprovider.Registry) {
	registrar.RegisterLibrary(NewSecretLibrary(registry))
}

// NewSecretLibrary creates the scriptling.secret library.
func NewSecretLibrary(registry *secretprovider.Registry) *object.Library {
	if registry == nil {
		registry = secretprovider.NewRegistry()
	}

	return object.NewLibrary(SecretLibraryName, map[string]*object.Builtin{
		"get": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 2, 3); err != nil {
					return err
				}

				alias, errObj := args[0].AsString()
				if errObj != nil {
					return errors.ParameterError("alias", errObj)
				}

				path, errObj := args[1].AsString()
				if errObj != nil {
					return errors.ParameterError("path", errObj)
				}

				field := ""
				if len(args) == 3 {
					field, errObj = args[2].AsString()
					if errObj != nil {
						return errors.ParameterError("field", errObj)
					}
				}

				value, err := registry.Resolve(ctx, alias, path, field)
				if err != nil {
					return errors.NewError("%s", err)
				}

				return &object.String{Value: value}
			},
			HelpText: `get(alias, path, field="") - Resolve a secret through a host-configured provider

Fetches a secret using the provider alias registered by the host application.
Scripts never see provider URLs, tokens, or other private configuration.

Parameters:
  alias - Registered provider alias (for example "vault" or "op")
  path - Provider-specific secret path or identifier
  field - Optional field to extract from a multi-value secret

Returns:
  Secret value as a string

Examples:
  import scriptling.secret as secret

  password = secret.get("prod_vault", "secret/data/app", "password")
  api_key = secret.get("op", "Engineering/api-key", "credential")`,
		},
	}, nil, "Provider-agnostic secret access using host-configured aliases")
}

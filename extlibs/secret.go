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
		"list": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 2, 2); err != nil {
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

				keys, err := registry.List(ctx, alias, path)
				if err != nil {
					return errors.NewError("%s", err)
				}

				elements := make([]object.Object, 0, len(keys))
				for _, key := range keys {
					elements = append(elements, &object.String{Value: key})
				}

				return &object.List{Elements: elements}
			},
			HelpText: `list(alias, path) - List keys at a path through a host-configured provider

Returns the key names at the given path for Vault, or item titles in a vault for 1Password.
Scripts never see provider URLs, tokens, or other private configuration.

Parameters:
  alias - Registered provider alias (for example "vault" or "op")
  path - Provider-specific path (Vault secret path, or 1Password vault name)

Returns:
  List of key or item name strings

Examples:
  import scriptling.secret as secret

  keys = secret.list("prod_vault", "secret/data/app")
  items = secret.list("op", "Engineering")`,
		},
	}, nil, "Provider-agnostic secret access using host-configured aliases")
}

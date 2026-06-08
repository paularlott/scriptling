package plugin

import (
	"encoding/json"

	"github.com/paularlott/scriptling/object"
)

const (
	ProtocolVersion = "1.0"
	NamespacePrefix = "plugin."

	valueNull     = "null"
	valueBool     = "bool"
	valueInt      = "int"
	valueFloat    = "float"
	valueString   = "string"
	valueList     = "list"
	valueDict     = "dict"
	valueRemote   = "remote"
	valueCallback = "callback"
)

func declaredLibraryName(name string) string {
	if len(name) > len(NamespacePrefix) && name[:len(NamespacePrefix)] == NamespacePrefix {
		return name[len(NamespacePrefix):]
	}
	return name
}

type Metadata struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Transport    string   `json:"transport,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Schema       Schema   `json:"schema"`
}

type Schema struct {
	Functions []FunctionSchema `json:"functions"`
	Classes   []ClassSchema    `json:"classes"`
	Constants []ConstantSchema `json:"constants"`
	Wrappers  []WrapperSchema  `json:"wrappers,omitempty"`
}

type FunctionSchema struct {
	Name        string   `json:"name"`
	Args        []string `json:"args,omitempty"`
	Description string   `json:"description,omitempty"`
	Wrapper     string   `json:"wrapper,omitempty"`
	Hidden      bool     `json:"hidden,omitempty"`
}

type ClassSchema struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Constructor FunctionSchema   `json:"constructor,omitempty"`
	Methods     []FunctionSchema `json:"methods,omitempty"`
}

type ConstantSchema struct {
	Name  string `json:"name"`
	Value Value  `json:"value"`
}

type WrapperSchema struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type Value struct {
	Type    string           `json:"type"`
	Value   any              `json:"value,omitempty"`
	Items   []Value          `json:"items,omitempty"`
	Entries map[string]Value `json:"entries,omitempty"`
	Remote  *RemoteRef       `json:"remote,omitempty"`
}

type RemoteRef struct {
	Library       string `json:"library"`
	Class         string `json:"class"`
	EnvironmentID string `json:"environment_id"`
	ID            string `json:"id"`
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type handshakeParams struct {
	Protocol     string   `json:"protocol"`
	Host         string   `json:"host"`
	HostVersion  string   `json:"host_version"`
	Transports   []string `json:"transports"`
	Capabilities []string `json:"capabilities"`
}

type handshakeResult struct {
	Protocol     string      `json:"protocol"`
	Transport    string      `json:"transport"`
	Library      libraryInfo `json:"library"`
	Capabilities []string    `json:"capabilities"`
	Schema       Schema      `json:"schema"`
}

type libraryInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type functionCallParams struct {
	EnvironmentID string           `json:"environment_id,omitempty"`
	Name          string           `json:"name"`
	Args          []Value          `json:"args,omitempty"`
	Kwargs        map[string]Value `json:"kwargs,omitempty"`
}

type objectNewParams struct {
	EnvironmentID string           `json:"environment_id,omitempty"`
	Class         string           `json:"class"`
	Args          []Value          `json:"args,omitempty"`
	Kwargs        map[string]Value `json:"kwargs,omitempty"`
}

type methodCallParams struct {
	EnvironmentID string           `json:"environment_id,omitempty"`
	ObjectID      string           `json:"object_id"`
	Method        string           `json:"method"`
	Args          []Value          `json:"args,omitempty"`
	Kwargs        map[string]Value `json:"kwargs,omitempty"`
}

type objectDestroyParams struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	ObjectID      string `json:"object_id"`
}

type callbackCallParams struct {
	EnvironmentID string           `json:"environment_id,omitempty"`
	CallbackID    string           `json:"callback_id"`
	Args          []Value          `json:"args,omitempty"`
	Kwargs        map[string]Value `json:"kwargs,omitempty"`
}

type callbackRef struct {
	fn  object.Object
	env *object.Environment
}

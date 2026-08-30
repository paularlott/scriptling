<?php
/*
 * A Scriptling plugin served over HTTP JSON-RPC, in plain PHP.
 *
 * Run it with PHP's built-in server:
 *
 *     php -S 127.0.0.1:8080 index.php
 *
 * then load it as a plugin:
 *
 *     scriptling --plugin http://127.0.0.1:8080 -c 'import plugin.phpdemo as d; print(d.greet("Ada"))'
 *
 * The protocol is two methods. The host POSTs a JSON-RPC request and this
 * file answers with a JSON-RPC result; the shapes mirror
 * docs/plugins/protocol on the website. Values travel as tagged objects
 * ({"type": "string", "value": "..."}) so scripts see native types.
 *
 * For https, terminate TLS in front of this server (a reverse proxy, or a
 * development certificate) and load the https:// URL; --plugin-insecure
 * accepts self-signed certificates.
 */

const LIBRARY_NAME = 'phpdemo';
const LIBRARY_VERSION = '1.0.0';

function value_string(string $s): array
{
    return ['type' => 'string', 'value' => $s];
}

function value_int(int $i): array
{
    return ['type' => 'int', 'value' => $i];
}

function value_dict(array $entries): array
{
    return ['type' => 'dict', 'entries' => $entries];
}

/** The handshake the host sends first; the library it declares is what
 *  scripts import (a bare name registers under plugin.<name>). */
function handshake(): array
{
    return [
        'protocol' => '1.0',
        'transport' => 'json',
        'library' => [
            'name' => LIBRARY_NAME,
            'version' => LIBRARY_VERSION,
            'description' => 'PHP example plugin served over HTTP',
        ],
        'capabilities' => [],
        'schema' => [
            'functions' => [
                ['name' => 'greet'],
                ['name' => 'echo'],
                ['name' => 'server_info'],
            ],
            'classes' => [],
            'constants' => [],
        ],
    ];
}

/** function.call dispatch: params carry the function name, positional args
 *  and kwargs, each a tagged value. */
function call_function(string $name, array $args, array $kwargs): array
{
    $arg = fn (int $i) => $args[$i]['value'] ?? null;
    $kwarg = fn (string $key) => $kwargs[$key]['value'] ?? null;

    switch ($name) {
        case 'greet':
            // The name from args or the name= kwarg; whichever the script used.
            $who = $arg(0) ?? $kwarg('name') ?? 'world';
            // An HTTP plugin owns its own environment: whatever this PHP
            // process was started with (the --plugin-env flag only applies
            // to executable plugins, which the host spawns).
            $from = getenv('PHPDEMO_FROM') ?: 'php';
            return value_string("Hello, {$who} (from {$from})");

        case 'echo':
            // Returns the first argument unchanged, any type: a round trip
            // of the value encoding rather than a re-implementation of it.
            if ($args === []) {
                throw new RpcError('echo takes one argument', -32602);
            }
            return $args[0];

        case 'server_info':
            return value_dict([
                'php' => value_string(PHP_VERSION),
                'sapi' => value_string(PHP_SAPI),
                'library' => value_string(LIBRARY_NAME . ' ' . LIBRARY_VERSION),
            ]);

        default:
            throw new RpcError("unknown function {$name}", -32602);
    }
}

/** A JSON-RPC error the dispatcher turns into an error response. */
final class RpcError extends Exception
{
    public function __construct(string $message, private readonly int $rpcCode)
    {
        parent::__construct($message);
    }

    public function rpcCode(): int
    {
        return $this->rpcCode;
    }
}

function respond(array $payload): never
{
    header('Content-Type: application/json');
    echo json_encode($payload, JSON_UNESCAPED_SLASHES);
    exit;
}

if (($_SERVER['REQUEST_METHOD'] ?? '') !== 'POST') {
    http_response_code(405);
    header('Allow: POST');
    respond(['error' => ['code' => -32600, 'message' => 'the plugin protocol speaks POST']]);
}

// Optional authentication: with PHPDEMO_TOKEN set, every request must carry
// it as a bearer token, so `--plugin-header Authorization=Bearer <token>`
// (or any proxy adding the header) can be exercised end to end. The built-in
// server exposes request headers as HTTP_* $_SERVER entries.
if (getenv('PHPDEMO_TOKEN') !== false) {
    $expected = 'Bearer ' . getenv('PHPDEMO_TOKEN');
    if (($_SERVER['HTTP_AUTHORIZATION'] ?? '') !== $expected) {
        http_response_code(401);
        respond(['jsonrpc' => '2.0', 'id' => null,
            'error' => ['code' => -32001, 'message' => 'missing or invalid bearer token']]);
    }
}

$request = json_decode(file_get_contents('php://input') ?: '', true);
if (!is_array($request) || !isset($request['id'], $request['method'])) {
    respond(['jsonrpc' => '2.0', 'id' => null,
        'error' => ['code' => -32600, 'message' => 'malformed JSON-RPC request']]);
}

try {
    $method = $request['method'];
    $params = $request['params'] ?? [];

    switch ($method) {
        case 'scriptling.handshake':
            $result = handshake();
            break;
        case 'function.call':
            $result = call_function(
                $params['name'] ?? '',
                $params['args'] ?? [],
                $params['kwargs'] ?? [],
            );
            break;
        case 'environment.open':
        case 'environment.close':
        case 'plugin.shutdown':
            // Lifecycle notifications the stdio transport sends; over HTTP
            // there is nothing to do, but answering keeps both transports
            // speaking the same protocol.
            $result = null;
            break;
        default:
            throw new RpcError("unknown method {$method}", -32601);
    }

    respond(['jsonrpc' => '2.0', 'id' => $request['id'], 'result' => $result]);
} catch (RpcError $e) {
    respond(['jsonrpc' => '2.0', 'id' => $request['id'],
        'error' => ['code' => $e->rpcCode(), 'message' => $e->getMessage()]]);
} catch (Throwable $e) {
    respond(['jsonrpc' => '2.0', 'id' => $request['id'],
        'error' => ['code' => -32000, 'message' => 'plugin error: ' . $e->getMessage()]]);
}

package mcp_manifest

const MCPServerSpecJSONSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "MCPServerSpec",
  "description": "Canonical MCP Server Contribution Spec",
  "type": "object",
  "required": ["schemaVersion", "transport"],
  "properties": {
    "schemaVersion": {"type": "integer", "const": 1},
    "transport": {
      "type": "object",
      "required": ["type"],
      "properties": {
        "type": {"type": "string", "enum": ["stdio", "streamable_http"]},
        "stdio": {
          "type": "object",
          "required": ["command"],
          "properties": {
            "command": {"type": "string", "minLength": 1, "pattern": "^[^\\s|&;(){}<>$\\\"'\\\\x00]+$"},
            "args": {
              "type": "array",
              "items": {"type": "string"},
              "maxItems": 128
            },
            "workDir": {"type": "string", "pattern": "^amitia://"},
            "env": {
              "type": "object",
              "properties": {
                "values": {"type": "object", "additionalProperties": {"type": "string"}},
                "secrets": {"type": "object", "additionalProperties": {"type": "string"}}
              }
            },
            "runtimeHint": {"type": "string", "enum": ["host", "node", "python"]}
          }
        },
        "remote": {
          "type": "object",
          "required": ["url"],
          "properties": {
            "url": {"type": "string", "format": "uri", "pattern": "^https?://"},
            "headers": {
              "type": "object",
              "additionalProperties": {
                "type": "object",
                "properties": {
                  "value": {"type": "string"},
                  "secretRef": {"type": "string"}
                }
              }
            },
            "auth": {
              "type": "object",
              "properties": {
                "type": {"type": "string", "enum": ["none", "bearer_token", "custom_headers", "stdio_env", "oauth"]},
                "secretRef": {"type": "string"},
                "oauth": {
                  "type": "object",
                  "properties": {
                    "providerRef": {"type": "string"},
                    "scopes": {"type": "array", "items": {"type": "string"}},
                    "clientRef": {"type": "string"}
                  }
                }
              }
            },
            "allowPrivateNetwork": {"type": "boolean"}
          }
        }
      }
    },
    "capabilities": {
      "type": "object",
      "properties": {
        "server": {
          "type": "object",
          "properties": {
            "tools": {"type": "string", "enum": ["disabled", "optional", "required"]},
            "resources": {"type": "string", "enum": ["disabled", "optional", "required"]},
            "prompts": {"type": "string", "enum": ["disabled", "optional", "required"]},
            "tasks": {"type": "string", "enum": ["disabled", "optional", "required"]},
            "completion": {"type": "string", "enum": ["disabled", "optional", "required"]},
            "logging": {"type": "string", "enum": ["disabled", "optional", "required"]}
          }
        },
        "client": {
          "type": "object",
          "properties": {
            "roots": {"type": "string", "enum": ["disabled", "optional", "required"]},
            "sampling": {"type": "string", "enum": ["disabled", "optional", "required"]},
            "elicitation": {"type": "string", "enum": ["disabled", "optional", "required"]},
            "tasks": {"type": "string", "enum": ["disabled", "optional", "required"]}
          }
        }
      }
    },
    "lifecycle": {
      "type": "object",
      "properties": {
        "autoStart": {"type": "boolean"},
        "restartPolicy": {"type": "string", "enum": ["never", "on_failure", "always"]},
        "maxRestartAttempts": {"type": "integer", "minimum": 0},
        "startupTimeout": {"type": "string", "pattern": "^[0-9]+(s|m|h)$"},
        "shutdownTimeout": {"type": "string", "pattern": "^[0-9]+(s|m|h)$"},
        "reconnect": {
          "type": "object",
          "properties": {
            "enabled": {"type": "boolean"},
            "maxAttempts": {"type": "integer", "minimum": 0},
            "backoff": {"type": "string", "enum": ["fixed", "exponential"]}
          }
        },
        "refreshOnReconnect": {"type": "boolean"}
      }
    },
    "security": {
      "type": "object",
      "properties": {
        "allowPrivateNetwork": {"type": "boolean"}
      }
    },
    "metadata": {
      "type": "object",
      "additionalProperties": {"type": "string"}
    }
  }
}`

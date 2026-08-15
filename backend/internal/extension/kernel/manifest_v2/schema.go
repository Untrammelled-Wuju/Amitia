package manifest_v2

const manifestV2Schema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["manifestVersion", "extension", "publisher"],
  "properties": {
    "manifestVersion": {"type": "integer", "const": 2},
    "placement": {"type": "string", "enum": ["cloud", "device", "hybrid"]},
    "extension": {
      "type": "object",
      "required": ["id", "name", "version"],
      "properties": {
        "id": {"type": "string", "pattern": "^[a-z0-9][a-z0-9-]*(\\.[a-z0-9-]+)+/[a-z0-9][a-z0-9-]*$"},
        "name": {
          "type": "object",
          "required": ["default"],
          "properties": {
            "default": {"type": "string", "minLength": 1},
            "translations": {"type": "object"}
          }
        },
        "description": {
          "type": "object",
          "properties": {
            "default": {"type": "string"},
            "translations": {"type": "object"}
          }
        },
        "version": {"type": "string", "pattern": "^(\\d+)\\.(\\d+)\\.(\\d+)(?:-[0-9A-Za-z-.]+)?(?:\\+[0-9A-Za-z-.]+)?$"},
        "license": {"type": "string"},
        "homepage": {"type": "string"},
        "repository": {"type": "string"},
        "categories": {"type": "array", "items": {"type": "string"}},
        "keywords": {"type": "array", "items": {"type": "string"}},
        "icon": {"type": "string"},
        "metadata": {"type": "object"}
      }
    },
    "publisher": {
      "type": "object",
      "required": ["id", "displayName"],
      "properties": {
        "id": {"type": "string", "minLength": 1},
        "displayName": {"type": "string", "minLength": 1},
        "trustLevel": {"type": "string", "enum": ["untrusted", "trusted", "verified", "official"]},
        "contact": {"type": "string"},
        "website": {"type": "string"}
      }
    },
    "compatibility": {
      "type": "object",
      "properties": {
        "minHostVersion": {"type": "string"},
        "maxHostVersion": {"type": "string"},
        "platforms": {"type": "array", "items": {"type": "string"}},
        "featureFlags": {"type": "array", "items": {"type": "string"}}
      }
    },
    "modules": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["id", "name", "type"],
        "properties": {
          "id": {"type": "string", "minLength": 1},
          "name": {
            "type": "object",
            "required": ["default"],
            "properties": {
              "default": {"type": "string", "minLength": 1},
              "translations": {"type": "object"}
            }
          },
          "description": {
            "type": "object",
            "properties": {
              "default": {"type": "string"},
              "translations": {"type": "object"}
            }
          },
          "type": {"type": "string", "enum": ["builtin", "javascript", "data_only", "wasm", "native", "service"]},
          "version": {"type": "string"},
          "runtime": {
            "type": "object",
            "required": ["type"],
            "properties": {
              "type": {"type": "string", "enum": ["javascript", "mcp", "workflow", "static", "wasm", "service"]},
              "entryPoint": {"type": "string"},
              "workerCount": {"type": "integer"},
              "timeout": {"type": "string"},
              "memory": {"type": "integer"},
              "permissions": {"type": "array", "items": {"type": "string"}},
              "capabilities": {"type": "object", "additionalProperties": {"type": "boolean"}},
              "env": {"type": "object"}
            }
          },
          "contributions": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["id", "kind", "name"],
              "properties": {
                "id": {"type": "string", "minLength": 1},
                "kind": {"type": "string", "enum": ["tool", "agent_skill", "workflow", "mcp_server", "provider", "hook", "event_subscription", "schedule", "background_task", "ui_page", "ui_panel", "ui_chat", "ui_context_action", "ui_desktop", "game_plugin", "desktop_pet_plugin"]},
                "name": {
                  "type": "object",
                  "required": ["default"],
                  "properties": {
                    "default": {"type": "string", "minLength": 1},
                    "translations": {"type": "object"}
                  }
                },
                "description": {
                  "type": "object",
                  "properties": {
                    "default": {"type": "string"},
                    "translations": {"type": "object"}
                  }
                },
                "version": {"type": "string"},
                "spec": {"type": "object"},
                "requiredPermissions": {"type": "array", "items": {"type": "string"}},
                "requiredScope": {"type": "array", "items": {"type": "string"}},
                "exposure": {
                  "type": "object",
                  "properties": {
                    "visibleByDefault": {"type": "boolean"},
                    "hiddenFromDiscovery": {"type": "boolean"},
                    "requiredRoles": {"type": "array", "items": {"type": "string"}}
                  }
                },
                "runtimeBinding": {
                  "type": "object",
                  "required": ["runtimeId"],
                  "properties": {
                    "runtimeId": {"type": "string", "minLength": 1},
                    "generation": {"type": "integer"}
                  }
                },
                "dependencies": {"type": "array", "items": {"type": "object"}}
              }
            }
          },
          "dependencies": {"type": "array", "items": {"type": "object"}},
          "compatibility": {
            "type": "object",
            "properties": {
              "minHostVersion": {"type": "string"},
              "platforms": {"type": "array", "items": {"type": "string"}}
            }
          },
          "policies": {
            "type": "object",
            "properties": {
              "isolation": {"type": "string"},
              "networkAccess": {"type": "boolean"},
              "fileSystemAccess": {"type": "boolean"}
            }
          },
          "placement": {"type": "string", "enum": ["cloud", "device"]},
          "deviceRequirements": {
            "type": "object",
            "properties": {
              "platforms": {"type": "array", "items": {"type": "string"}, "minItems": 1, "maxItems": 16, "uniqueItems": true},
              "architectures": {"type": "array", "items": {"type": "string"}, "minItems": 1, "maxItems": 16, "uniqueItems": true},
              "minAppVersion": {"type": "string"},
              "minRuntimeVersion": {"type": "string"},
              "requiredFeatures": {"type": "array", "items": {"type": "string", "minLength": 1, "maxLength": 128}, "minItems": 1, "maxItems": 64, "uniqueItems": true}
            }
          },
          "providedCapabilities": {
            "type": "array",
            "items": {
              "type": "object",
              "required": ["id"],
              "properties": {
                "id": {"type": "string", "minLength": 1, "maxLength": 256},
                "version": {"type": "string"},
                "metadata": {"type": "object"}
              }
            },
            "maxItems": 256
          },
          "provider": {
            "type": "object",
            "properties": {
              "id": {"type": "string", "minLength": 1, "maxLength": 256},
              "priority": {"type": "integer", "minimum": -100000, "maximum": 100000},
              "labels": {
                "type": "object",
                "additionalProperties": {"type": "string"},
                "maxProperties": 64
              },
              "metadata": {"type": "object"}
            }
          }
        }
      }
    },
    "dependencies": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["type", "id"],
        "properties": {
          "type": {"type": "string", "enum": ["extension", "module", "mcp", "provider", "host_api"]},
          "id": {"type": "string", "minLength": 1},
          "version": {"type": "string"},
          "optional": {"type": "boolean"},
          "reason": {"type": "string"}
        }
      }
    },
    "permissions": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": {"type": "string", "minLength": 1},
          "reason": {"type": "string"},
          "required": {"type": "boolean"},
          "scope": {"type": "string"}
        }
      }
    },
    "resources": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "type", "path"],
        "properties": {
          "id": {"type": "string", "minLength": 1},
          "type": {"type": "string", "minLength": 1},
          "path": {"type": "string", "minLength": 1},
          "hash": {"type": "string"},
          "size": {"type": "integer"}
        }
      }
    },
    "lifecycle": {
      "type": "object",
      "properties": {
        "autoUpdate": {"type": "boolean"},
        "backgroundTasks": {"type": "boolean"},
        "networkAccess": {"type": "boolean"},
        "isolation": {"type": "string"},
        "sandbox": {"type": "boolean"}
      }
    },
    "integrity": {
      "type": "object",
      "required": ["algorithm", "contentTreeHash"],
      "properties": {
        "algorithm": {"type": "string", "minLength": 1},
        "contentTreeHash": {"type": "string", "minLength": 1},
        "fileHashes": {"type": "object"}
      }
    },
    "development": {
      "type": "object",
      "properties": {
        "developerMode": {"type": "boolean"},
        "hotReload": {"type": "boolean"},
        "sourceMaps": {"type": "boolean"},
        "testEntry": {"type": "string"},
        "watchPaths": {"type": "array", "items": {"type": "string"}}
      }
    }
  }
}`

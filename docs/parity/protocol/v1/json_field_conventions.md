# JSON Field Conventions

## Overview
Standard JSON field naming and structure conventions for all protocol files.

## Field Naming
- **Style**: lowerCamelCase
- **Examples**: capabilityId, toolId, behaviorKey, createdAt

## Standard Fields
| Field | Type | Description |
|-------|------|-------------|
| id / *Id | string | Unique identifier |
| name | string | Human-readable name |
| description | string | Detailed description |
| type | string | Type classification |
| version | string | Version identifier |
| createdAt | string (ISO 8601) | Creation timestamp |
| updatedAt | string (ISO 8601) | Last update timestamp |
| status | string | Current status |
| metadata | object | Additional metadata |

## Array Fields
- Use plural nouns: capabilities, tools, errors
- Example: \"capabilities\": [...]

## Boolean Fields
- Use is/has/can prefix
- Example: \"isActive\", \"hasPermission\", \"canExecute\"

## Numeric Fields
- Use descriptive suffixes when needed
- Example: \"totalCount\", \"minValue\", \"maxValue\"

## Date/Time Fields
- Use ISO 8601 format
- Use At suffix: createdAt, updatedAt, frozenAt

## Required vs Optional
- Required fields: id, type, status
- Optional fields: description, metadata, tags

# Naming Conventions

## Overview
All identifiers follow strict naming conventions to ensure consistency and readability.

## Capability ID
- **Format**: UPPER_DOT (e.g., CAP.FILE.READ.CONTENT)
- **Domain**: UPPER_SNAKE_CASE
- **Action**: UPPER_SNAKE_CASE
- **Object**: UPPER_SNAKE_CASE

## Tool ID
- **Format**: lower.dot_snake (e.g., tool.file.read_content)
- **Domain**: lowercase
- **Action**: lowercase
- **Object**: snake_case

## Permission ID
- **Format**: UPPER_DOT (e.g., PERM.FILE.WORKSPACE.READ)
- **Domain**: UPPER_SNAKE_CASE
- **Resource**: UPPER_SNAKE_CASE
- **Level**: UPPER_SNAKE_CASE

## Error Code
- **Format**: UPPER_SNAKE (e.g., ERR_TOOL_INVALID_ARGUMENT_1001)
- **Domain**: UPPER_SNAKE_CASE
- **Category**: UPPER_SNAKE_CASE
- **Number**: 5-digit integer

## JSON Field
- **Format**: lowerCamelCase (e.g., capabilityId, toolId, behaviorKey)
- **Arrays**: plural nouns (e.g., capabilities, tools)
- **Booleans**: is/has/can prefix (e.g., isAvailable, hasPermission)

## Event/Hook/Trigger ID
- **Format**: lower.dot (e.g., event.agent.conversation.started)
- **Segments**: domain.object.state

## Provider ID
- **Format**: lower.dot (e.g., provider.model.openai)

## Service ID
- **Format**: lower.dot (e.g., service.core.agent)

## Runtime ID
- **Format**: lower.dot (e.g., runtime.node.win32)

## Resource URI
- **Scheme**: amitia://
- **Path**: lowercase with slashes
- **Query**: lowerCamelCase

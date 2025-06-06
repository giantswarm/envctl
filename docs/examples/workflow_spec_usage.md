# Using the workflow_spec Tool

The `workflow_spec` tool helps agents understand how to create and manage workflows in envctl. This document shows examples of using this tool.

## Getting the Full Specification

To get comprehensive information about workflows:

```json
{
  "tool": "workflow_spec",
  "arguments": {
    "format": "full"
  }
}
```

This returns:
- Complete YAML schema for workflows
- A minimal workflow template
- Example workflows
- Template syntax documentation

## Getting Just the Schema

To understand the structure and required fields:

```json
{
  "tool": "workflow_spec",
  "arguments": {
    "format": "schema"
  }
}
```

## Getting a Template

To get a starting point for creating a new workflow:

```json
{
  "tool": "workflow_spec",
  "arguments": {
    "format": "template"
  }
}
```

## Getting Examples

To see practical workflow examples:

```json
{
  "tool": "workflow_spec",
  "arguments": {
    "format": "examples"
  }
}
```

## Agent Workflow Creation Example

Here's how an agent might use `workflow_spec` to create a new workflow:

1. First, get the template:
   ```json
   {
     "tool": "workflow_spec",
     "arguments": {"format": "template"}
   }
   ```

2. Modify the template based on requirements

3. Validate the workflow:
   ```json
   {
     "tool": "workflow_validate",
     "arguments": {
       "yaml_definition": "# your workflow YAML here"
     }
   }
   ```

4. Create the workflow:
   ```json
   {
     "tool": "workflow_create",
     "arguments": {
       "yaml_definition": "# your validated workflow YAML here"
     }
   }
   ```

## Benefits for Agents

- No need to hardcode workflow format knowledge
- Always get up-to-date schema information
- Examples help understand common patterns
- Self-documenting API reduces errors 
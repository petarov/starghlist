starghlist
===============

Manage your GitHub starred repositories using MCP.

# Usage

You will need a valid GitHub [PAT](https://github.com/settings/personal-access-tokens).

The server uses SSE protocol transport type.

To start on `localhost:24240` run:

    GITHUB_TOKEN=<your PAT> go run *.go 

# Clients

## LMStudio

Works pretty well with `gemma-3-12b` and `gpt-oss-20b`, and kind of ok with `llama-3.1-8b-instruct`.

```json
{
  "mcpServers": {
    "mcp-starghlist": {
      "url": "http://127.0.0.1:24240"
    }
  }
}
```

## Cline

Pretty much flawlessly with `claude-sonnet-4`.

```json
{
  "mcpServers": {
    "mcp-starghlist": {
      "url": "http://127.0.0.1:24240",
      "disabled": false,
      "autoApprove": [],
      "timeout": 30
    }
  }
}
```

# License

[MIT](LICENSE)
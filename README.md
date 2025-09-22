starghlist
===============

<p align="left">
<img src="assets/ghlist.png" alt="Bloodworm vector image" width="180"/>
</p>

This is a MCP server that is supposed to help you manage your GitHub starred repositories and lists.

<sub>In Bulgarian, „глист“ or "glist" means an intestinal parasitic worm (roundworm).</sub>

# Tools

- `getStarred`: Get the users's starred repositories.
- `addStar`: Star one or more repositories.
- `removeStar`: Unstar one or more repositories.
- `getListNames`: Get all of the user's lists.

# Prompts

TODO ...


# Setup

You will need a valid GitHub [PAT](https://github.com/settings/personal-access-tokens).

The server uses SSE protocol transport type. To start on `localhost:24240` run:

    GITHUB_TOKEN=<your PAT> go run *.go 

# Clients

```json
{
  "mcpServers": {
    "mcp-starghlist": {
      "url": "http://127.0.0.1:24240"
    }
  }
}
```

### LMStudio

Works pretty well with `gemma-3-12b` and `gpt-oss-20b`, and kind of ok with `llama-3.1-8b-instruct`.

### Cline

Pretty much flawless with `claude-sonnet-4`.

# License

[MIT](LICENSE)

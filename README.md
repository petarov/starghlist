<p align="left">
<img src="assets/ghlist.png" alt="Bloodworm vector image" width="180"/>
</p>

starghlist
===============

starghlist<sup>*</sup> is an MCP server designed to help you manage your GitHub starred repositories and lists.

You can use an AI agent to view, sort, and filter repositories, as well as unstar those that are no longer relevant. 
The AI agent can also suggest how to categorize your starred repositories based on the GitHub lists you’ve created. 
Adding or removing repositories from lists is not yet supported—it’s a work in progress.

<sub><sup>*</sup>In Bulgarian, „глист“ or "glist" means an intestinal parasitic worm (roundworm).</sub>

# Tools

- `getStars`: Get starred repositories either by you or somebody else.
- `addStar`: Star one or more repositories.
- `removeStar`: Unstar one or more repositories.
- `getStarListNames`: Get all of your starred repository lists.
- `addStarToList`: Add starred repositories to a list.
- `removeStarFromList`: Remove starred repositories from a list.

# Setup

If you don't care about managing lists, then you can use a GitHub [PAT](https://github.com/settings/personal-access-tokens).
The permissions `Profile` and `Starring` should be enough.

If you want to manage lists, this currently works only with a GitHub [classic PAT](https://github.com/settings/tokens). 
Make sure you check the `user` scope, not just its leafs but the whole scope.

The server uses the Streamable HTTP transport. To start on `localhost:24240` run:

    GITHUB_TOKEN=<your PAT> go run *.go 

Alternatively use the binary and specify a port:

    GITHUB_TOKEN=<your PAT> ./starghlist-1.0-darwin-aarch64 -host '[::1]' -port 8080

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

Works pretty well with `gpt-oss-20b` and `gemma-3-12b`, and kind of ok with `llama-3.1-8b-instruct`.

### Cline

Pretty much flawless with `claude-sonnet-4`.

# Prompts

Sample prompts tested with `gpt-oss-20b`:

`give me my first starred 10 github projects in a table`

`give me 5 starred projects by the user petarov`

`star the repo petarov/starghlist`

`remove the star from petarov/starghlist`

`give me a list of all my starred repository lists`

`add the repo petarov/starghlist to the list ai-dev`

`remove the repo petarov/starghlist from the ai-dev list`

# License

[MIT](LICENSE)

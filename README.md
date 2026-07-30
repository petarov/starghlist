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
- (TODO) `addStarToList`, 
- (TODO) `removeStarFromList`

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

Works pretty well with `gpt-oss-20b` and `gemma-3-12b`, and kind of ok with `llama-3.1-8b-instruct`.

### Cline

Pretty much flawless with `claude-sonnet-4`.

# Prompts

Sample prompts tested with `gpt-oss-20b`:

`give me my first starred 10 github projects in a table`

`give me 5 starred projects by the user petarov`

`star the repo petarov/starghlist`

`remove the star from petarov/starghlist`

`give me the names of all my starred repository lists`

# License

[MIT](LICENSE)

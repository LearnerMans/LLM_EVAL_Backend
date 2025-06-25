## File Dependencies

Here is a list of the Go files in this project and their direct internal dependencies:

*   **`main.go`**:
    *   `agent/agent.go`
    *   `llm/gemini_client.go` (for `llm.CurrentState`, `llm.HistoryItem` types)
    *   `db/db.go` (currently commented out in `main.go` but intended for use)

*   **`agent/agent.go`**:
    *   `knovvu/knovvu_client.go`
    *   `llm/gemini_client.go` (for `llm.LLMInput`, `llm.CurrentState`, `llm.HistoryItem`, `llm.GenerateContentREST`)

*   **`db/db.go`**:
    *   No internal project dependencies.

*   **`knovvu/knovvu_client.go`**:
    *   No internal project dependencies.

*   **`llm/gemini_client.go`**:
    *   `llm/prompts.go` (for `SystemPrompt`)

*   **`llm/openai_client.go`**:
    *   `llm/gemini_client.go` (for shared types: `LLMInput`, `LLMOutput`)
    *   `llm/prompts.go` (for `SystemPrompt`)

*   **`llm/prompts.go`**:
    *   No internal project dependencies.

---

## Dependency Diagram (Mermaid Flowchart)

```mermaid
graph TD
    subgraph "Main Package"
        Main["main.go"]
    end

    subgraph "Agent Package"
        Agent["agent/agent.go"]
    end

    subgraph "DB Package"
        DB["db/db.go"]
    end

    subgraph "Knovvu Client Package"
        KnovvuClient["knovvu/knovvu_client.go"]
    end

    subgraph "LLM Package"
        LLMGemini["llm/gemini_client.go"]
        LLMOpenAI["llm/openai_client.go"]
        LLMPrompts["llm/prompts.go"]
    end

    %% Dependencies
    Main --> Agent
    Main --> LLMGemini
    Main -.-> DB  -- Commented out in code --> DB

    Agent --> KnovvuClient
    Agent --> LLMGemini

    LLMGemini --> LLMPrompts
    LLMOpenAI --> LLMGemini %% For shared types
    LLMOpenAI --> LLMPrompts

    %% Styling (optional, but makes it clearer)
    classDef default fill:#f9f,stroke:#333,stroke-width:2px;
    class Main,Agent,DB,KnovvuClient,LLMGemini,LLMOpenAI,LLMPrompts default;
```

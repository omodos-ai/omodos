# ggo: AI Orchestrator & Time Machine

`ggo` is a headless, Git-aware Unix orchestrator designed for expert-level software architecture, coding, and knowledge management. It operates as a unified hub, binding your local filesystem, OS clipboard, and Git history directly into a conversational AI interface.

## 🚀 Getting Started

Start the interactive terminal chat (REPL) by simply running the binary from any enrolled project directory:

```bash
ggo
```


This drops you into the ggo> prompt.

## The Emacs Interface & / Commands

If you are using the provided Emacs package (plugins/ggo.el), you interface with the daemon via ggo-chat-mode.

Because Emacs pipes your input directly to the daemon, conversational prompts are typed normally, but control commands must be prefixed with a forward slash (/). Example in Emacs:

Write a function to hash a string. -> Sent as a conversational prompt.

/wf -> Sent to the control pipe to execute the wf (write-files) command.

(Note: In the raw terminal REPL, you do not need the / prefix. You simply type wf or inc).

🛠️ Command Reference
Whether you are typing them directly into the terminal or prefixing them with / in Emacs, here are the core commands available in the ggo environment.

Context & Chat Management
cc <chat-name>: Change or create a new active chat context (e.g., cc debugging_auth).

inc <path...>: Intelligently ingest one or more files or directories into the AI's permanent Read-Only baseline context. (Binary files and .git folders are automatically skipped).

clear: Clears the active chat context (Stubbed).

Prompting & OS Integration
p: Enters Multi-line Prompt mode. Useful for pasting large blocks of text directly into the terminal without triggering execution on the first newline.

ps: Prepend Snippet. Automatically grabs the contents of your OS clipboard, allows you to type a prompt above it, and sends the combined payload to the LLM.

cr: Copy Response. Extracts the code block (or cleaned markdown) from the LLM's last response and copies it directly to your OS clipboard.

cw <path>: Copy to File. Extracts the clean response and writes it directly to the specified file path.

wf (or write-files): Macro Edit. Scans the LLM's last response for strict XML <file> mutations and automatically applies the overwrites to your local disk, wrapped in a Git Time Machine transaction.

The Time Machine (Git Orchestration)
ggo automatically intercepts file modifications and wraps them in mathematical Git states called "Shadows".

gr <path>: Git Add Repo. Enrolls a local Git repository into the Time Machine's tracking list.

gl: Git Ledger. Lists all chronologically recorded Time Machine transactions (anchors).

gs <index>: Git Sync. Travels through time. Instantly restores the working tree of all enrolled repositories to the exact mathematical state of the specified ledger index.

git-remote <push|pull|clone>: Executes remote synchronization operations against the central vault and all enrolled code repositories.

Environment & Tuning
set <field> <value>: Dynamically updates project or global configuration (e.g., set temperature 0.1, set model 1).

show <global|project|costs|params|roles>: Displays the requested configuration, cost ledger, or parameter tuning table.

list <projects|models>: Lists all tracked ggo workspaces or currently available Gemini models.

add role <name> <file_or_desc>: Dynamically creates and activates a new behavioral persona from a string or a markdown file.

💡 Example Workflow
Here is how a standard development loop looks using ggo:

Enroll your codebase:
gr .

Inject context:
inc ./pkg/auth/auth.go

Change the chat thread:
cc refactoring_auth

Prompt the AI:
I need you to rewrite the JWT validation logic to use ECDSA. Output the result using your strict XML <file> schema.

Apply the resulting code:
wf (ggo scans the output, backs up your current state via the Time Machine, and overwrites auth.go).

Oh no, the code broke! Rollback:
gl (Look at the transaction index) -> gs 0 (Revert to the exact state before wf was run).

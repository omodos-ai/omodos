---
number headings: auto, first-level 2, max 6, skip ^skipped, 0.1.1.1.1
---

#  ggo - a stateful go cli for ML collaboration 

## 1 Remaining Todo 

- [ ] grab updated context, package up for sending
- [ ] update context
- [ ] introduce chats subdirectory with `<name>.md` chat files
- [ ] fast resume with Gemini context (Hydration Engine)
- [ ] file ai records in source
- [ ] screen response to emacs for control characters
- [ ] snapshot, write files
- [x] fix the time machine test ✅ 2026-04-05
- [ ] copy snippets from response to pastebuffer (OS Clipboard integration)
- [x] context-inc, inc: for context ✅ 2026-04-05
- [x] git-add-repo, gr: register a time machine repo ✅ 2026-04-05
- [ ] parse a chat.md file into source records and add into the source tree `ggo mdchat-pr-resp <ingest file> <source dir>`
- [ ] chat buffer with emacs control
- [ ] mark yank in terminal
- [ ] sync shadow and central repo to GH
- [ ] responses with snippets automatically placed into the paste buffers by ggo
- [ ] deprecate `<export date>` from the `*-pr-resp` commands. 
- [ ] chats subdir in `~/.ggo/project/chats/<chatname.md>`
- [ ] Create mark/yank shell commands to greatly ease creating pb snippets
- [ ] Implement the `parseChatHistory (ggo mdchat-pr-resp)` Hydration Engine to natively construct the `genai.History` object from the active markdown ledger.
- [ ] Fix the Time Machine untracked-file restoration edge cases (without using destructive resets).
- [ ] Write the `Context Injector` to dynamically gather `git diff` outputs and explicitly included text at the moment of dispatch.
- [ ] Solidify the `GEMINI.md` system prompt instructing the LLM on `<file path>` output usage.
- [ ] Write the autonomous file-writer interceptor for macro-edits.
- [ ] Integrate `github.com/atotto/clipboard` to handle automatic pushing of command/code payloads to the user's paste buffer.
- [ ] Strip control characters prior to clipboard injection or markdown ledger appending.


## 2 Architecture & Design Specification (v1.1)

Ggo has the following purposes.  
- bulk data management across development projects, chats and notebookLM
- frictionless Unix command style assistance with authoring and coding
- safety of state (time machine like) if AI is making modifications to projects
- executing chats in users favourite editors.


IDE plugins can be built easily with ggo, but this is not our focus. Ggo does not replace smart substitution mechanisms such as offered by Cursor with LSP.

### 2.1 The Unified Binary (Cobra CLI) 

ggo is a single, statically compiled Go executable that replaces an entire ecosystem of scattered Python and Bash scripts.

It leverages 
- cobra
- readline
- viper
- auto completions for commands, paths, fields found in yaml files

### 2.2 ai steps and ai records

Every ggo interaction is called an ai step.  Each ai step creates a timestamped ai record.  These records can be reproduced from e.g. ChatGPT exports, Google My Activity Takeout for Gemini and ggo's own markdown chat files. 

The records are kept in a `<source>/yyyy/mm` directory structure.

Append-Only Rule: The `<source>` export log ai records act as an immutable ledger. They are strictly append-only. If `gs` is used to roll back the shadow git repository, the log is never reverted; instead, a new entry is appended recording the rollback event to preserve the complete narrative.


File naming specification: 
```
<source_dir>/YYYY/MM/<YYYYMMDDThhmmss>-<Subject_Slug_Up_To_20_Chars>.md
```
Markdown AI interaction format specification: 
```Markdown
## AI Interaction
### Date
2026-04-03T01:22:30Z
### Chat
<chat name>
### Source
ggo-cli / Gemini Pro
### Includes
-
- 
### Current working directory
<path>
### Subject
<Subject Slug derived from the first 5 words of the prompt>
### Context
<List of /inc files or filewatcher diffs applied during this turn>
### Prompt
<The exact raw text submitted by the user or piped from a shell command>
### Response
<The exact raw markdown returned by the Gemini API>

```

### 2.3 Macro and Micro Edits (XML Schema & OS Clipboard)

`ggo` treats file modification purely as a text-generation API problem.
- **Macro-Edits:** For large autonomous file writes, `ggo` strictly enforces a `<file path="...">` XML schema in the `GEMINI.md` prompt. `ggo` intercepts this structured output and safely writes the file to the filesystem natively.
- **Micro-Edits:** For smaller surgical edits, bash scripts, or refactors, `ggo` isolates the generated code blocks and writes them to the OS Clipboard. The user's native editor or terminal consumes the clipboard output.


## 3 Bulk AI data processing

ggo is a single command line utility that manages AI interactions for development, chat, notebookLM archival queries in a single place.  Its key architectural drivers are:

- smoothly transition between the tools with round trip data to e.g. get a summary out of NotebookLM and use it with all referenced prompts as a new feed for chat.
- For development, one works in projects with a collection of repos and chats attached to it. The tool removes the "copy-paste" cycle by preparing paste buffers in the system clipboard (with history, like Maccy) and it can directly write replacement files to the disk
- Work in a transactional model: each _ai step_ with the AI that delivers information or changes data in your projects, is timestamped, creates a small transaction across shadow repositories that retain the old state, remember exactly (i) the modified state of the working repo's before the AI changed them (ii) the modified state after the ai modified the working repos.  There is precise rollback and restore possible in the working repos (including staged files, branch tags etc) by using the systems' shadow records.   This avoids uncertainty and confusion. 
- the tool runs chat in the users favourite editor with a data and control pipe.  For each ai step an _ai record_ is generated that is timestamped and becomes available in an archive
- The tool avoids doing anything that existing tools already do well, like clipboard managing, having special commands in editors.  It is a buffer between the normal developer toolchaing and the AI.  It has less ability than e.g. Cursor in terms of managing intelligent substitutions across a number of files, but far more capability than Cursor in terms of managing a context of reasoning, design and coding information and gradually progress through implementation and bug fixing with this.

### 3.1 Streamlined execution

The `ggo` batch pipeline unifies raw `.zip` exports from Google Takeout (Gemini) and ChatGPT into chronologically ordered Markdown ledgers and NotebookLM-ready chunks.

**1. Download & Placement**

Request your data exports from Google Takeout and OpenAI. Place the resulting `.zip` files directly into your configured ingest directory. Do not manually extract them.

**2. Global Path Configuration**

Ensure your global directories are configured in the REPL (these default to `~/.ggo/config.yaml`):

Plaintext

```
ggo> set ingest_dir ~/.ggo/ingest     # Where the .zip files live
ggo> set source_dir ~/.ggo/source     # Where parsed Markdown ledgers are saved
ggo> set chunks_dir ~/.ggo/chunks     # Where aggregated chunk files are 
```

**3. Set the Export Date**

LLM providers often date their exports inconsistently (e.g., ChatGPT exports may be dated two days prior). Unify the pipeline under a single date using the dedicated CLI command:

Bash

```
$ ggo set-export-date YYYY-MM-DD
```

**4. Execute the Pipeline**

Run the two master commands. `unzip` safely extracts and organizes the archives based on the export date. `chunks` automatically triggers both parsers (Gemini and ChatGPT) and aggregates the output into `<350k` word chronological chunks.

Bash

```
$ ggo unzip
$ ggo chunks
```
#### 3.1.1 Older manual commands - probably not functional anymore

    - `ggo` (no arguments): Drops directly into the interactive stateful REPL.  The repl is discussed later in this document.
    - `ggo unzip [-f] <ingest dir> <date>`: Runs the Takeout/ ChatGPT extraction logic. 
    - `ggo cg-pr-resp [-f] <ingest dir> <source dir> <export date>`: creates prompt replies pairs 
    - `ggo gemini-pr-resp [-f] <ingest dir> <source dir> <export date>`: creates prompt replies pairs 
    - `ggo chunks <date> <source-dir> <chunks-dir>`: creates prompt reply pairs in `<export-dir>/<date>` (to avoid overwriting), and chunks them for import in NLM. 
    - `ggo extract <source dir> <infile> <outfile>`: creates a full summary of all AI interactions associated with a NLM report, based on timestamps 
    - `ggo sync`: Triggers a git push/pull on your ~/ai/chats/source repository. The `<source>` directory is append only. 

## 4 Ggo for authoring and development

### 4.1 The ggo purpose and core workflows

The ggo suite has been built to support AI interactions with an automatically growing included context and chats. The creative aspect for which ggo was built is detailed AI assisted intellectual editing, authoring (bug fixing, paper authoring), and creating new projects. It is assumed users have working repos for their work, and ggo will leave them as unaffected as is possible.

Ggo and the AI backend interact in just a few ways with the users system:

- ai steps however small create ai records for provenance and later reprocessing (e.g. with NotebookLM).
- ggo can write source files the AI generates (through an explicit locally executed command). Before it does, it saves state in the shadow repos.
- ggo can put snippets, like patches, commands into the systems paste buffers (we assume there is a system with a history ring of "copied" text).
- ggo executes a chat in the users favourite editor through a data and control pipe, which appears superior for most development work to web tools and applications like ChatGPT and Gemini. We screen LLM text for accidental control characters that could be unsafe in text buffers.
    

As such, ggo removes friction and provides safety. It is not a cursor like plugin, it is not a trivial to use web page, but it can be 100% keyboard driven.

#### 4.1.1 **ggo chats** 

For ai steps generated by ggo, ai records are created both in the global `<source>` directory and in the `~/.ggo/<project>/chats/chatname.md` file in the ggo configuration repo, so that format changes or errors in the bulk command tools can be recovered. A command can re-create the global `<sources>` with the chat records (`ggo mdchat-pr-resp <chatname>`).

#### 4.1.2 **The Context Injector** 

ggo intentionally avoids running complex background fsnotify daemons to prevent noisy auto-save triggers. Instead, it uses a Just-In-Time Context Injector. When a prompt is submitted, ggo dynamically bundles:

- All explicitly attached read-only context (inc / context-include files).
- Files changed in directories in the working repos.
- Files in the central repo's .gitignore are not updated. This guarantees the context sent to the API is perfectly synchronized with the user's explicit intent at the exact moment of dispatch. We avoid context bloat by not injecting entire repos into the context, even though the entire repos do get shadow repos (that is local).
    

#### 4.1.3 **Macro and Micro Edits (XML Schema & OS Clipboard)** 

ggo treats file modification purely as a text-generation API problem.

- **Macro-Edits:** For large autonomous file writes, ggo strictly enforces a `<file path="...">` XML schema in the GEMINI.md prompt. ggo intercepts this structured output and safely writes the file to the filesystem natively.
    
- **Micro-Edits & The OS Clipboard:** ggo acts strictly as an orchestrator and decouples entirely from editors and terminals by utilizing the OS Clipboard (e.g., Maccy, atotto/clipboard) as the universal pipe. ggo does not spawn background shells, handle native ediff commands, or manage complex IPC sockets - with one exception: running a chat in the users favourite editor through a pipe (and with a control channel). If the LLM generates a code block or shell command, ggo pushes the raw string directly to the OS clipboard. The user manually pastes it into their terminal or editor of choice, maintaining absolute control. Before ggo writes AI responses to chat buffers or clipboards, it strictly sanitizes the output of terminal escape sequences and Emacs control codes to prevent environment corruption.
    
#### 4.1.4 **The Core Workflows**

- **Workflow 1: The Macro Generator:** Action: You prompt ggo to generate 3 new files based on a design pattern. Result: The LLM streams back the XML payload. ggo natively parses the `<file path="...">` blocks and writes the files to disk, immediately enveloping the change in a Time Machine ledger update.
    
- **Workflow 2: The Micro-Edit & Clipboard Delivery:** Action: You prompt ggo for the 5 shell commands needed to restructure a directory. Result: The LLM responds. ggo strips out any dangerous control characters, logs the response to the .md chat ledger, and pushes the bash block directly into your OS Clipboard for manual pasting.
    
- **Workflow 3: The Time Machine Reversion:** Action: The AI generates a buggy rewrite and you notice it immediately. Result: You run `gs 0`. ggo reverts the target codebase to the precise foundation hash and applies the original pristine overlay via rsync. The chat log in `.ggo` persists to record the failure.
    

### 4.2 Understanding the ggo concepts and configuration and data

ggo works on projects. Each project has further configurations. All projects and their suspected location is listed globally under `~/.ggo` so that projects can be listed and found. Ggo can be run from anywhere and can trivially be amended to work remotely. The configuration of ggo is organised as follows:

- project configuration include lists of included context, shadow repos, chat names, last used chat (fore resumption), ai model and other variables.
- a project has included context given to Gemini (context-include/inc).
- a project has shadow repositories acting as a time machine backup (git-add-repo/gr).
- the ggo central repo is an git-lfs repo under `~/.ggo/<project name>`. This repo is not rolled back with time machine state, but does receive frequent commits, e.g. when included context state is updated and needs to be re-initialized. The reason this is done is that projects can be reloaded at a past point (e.g. at a point where something started to go wrong).
    

Projects contain:

- a per project configuration file.
- a local git-lfs repo which holds: per project configuration: its repos, context, chats, model, resumption state; all prompts, responses, and actual context state given to the LLM, timestamped; commit transactions updated after every interaction.
- It maintains GEMINI.md instructions so that your interaction is along the style you want.
    

For interactions between gemini chat, ChatGPT and the ggo generated chats various directories contain important state. All variables are held under `~/.ggo/`. Project directories contain no ggo configuration or state, hence ggo can be run from any location, load a project, or work (with some code changes) with remote shells. ggo has no concept of a working directory, but anchors pathnames in the calling users home directory to facilitate using it on other systems.

### 4.3 Detailed descriptions of the configuration data

The state is summarised as follows: the global configuration has: api key file names (not keys), global ai directory locations. per project steate.

### 4.4 **Configuration & State Managing configuration**

- `config-set [project] [field:value]]` If project is omitted a global variable is set. To set the project name and its directory two arguments must be passed after the `:`.
- `config-show [project]` - show global or project config variables.
- `pl` - project list - list all projects and their suspected directories. If `field:value` is omitted, a default value is set. All configurations and pathnames are verified upon loading and upon writing with Viper.
    

#### 4.4.1 **Global configuration (`~/.ggo/config.yaml`)** Specifies:

- `api_key_path` - file with gemini API key
- `github_account`
- `source_dir`
- `ingest_dir`
- `chunks_dir`: # here AI created snippets are stored (we don't need this it's in the ai record)
- `files_dir`: # here AI created files are stored (we don't need this, these are in the ai record)
- `editor` # we don't need this I think
- `quick-editor` # we don't need this
- `projects` array, project-name: `<last dir seen at>`
    

#### 4.4.2 Project Configuration 

(`.ggo/project.yaml`)** Configuration is local to the project directory, allowing graceful handling of remote execution and customized tooling per codebase. Directory structure:

```
~/
	.ggo/
		config.yaml # global configuration, api key file, <sources> etc.
		ggoai-<project-name>/  # the .ggo internal lfs repo
			project.yaml # project configuration, model, context includes, repos, resumption data
			GEMINI.md
			chats/chatname.md
			includes    # context includes
		shadow-repos/   # include commands ingest, their .git is noted
			path-repo1
			path-repo2
```

YAML

YAML

```
project_name: "ggo"
model_name:
shell_cmd: "bash" # Can be swapped to "ssh -t dev-vm 'bash'" for remote work
diff_cmd: "emacsclient -c --eval '(ediff-files \"{actual}\" \"{suggestion}\")'"
includes: [dir1, dir2, ... , file1, .. file2]
chat: [chat1, chat2, ....]
last_chat:  chat5
last_timestamp: 
gemini_remote_context_restore_token:
```

### 4.5 The Time Machine 

To provide safety ggo creates:

- a central repo, a git lfs repo with context, configurations, chat history.
- shadow repos to have time machine like backup restore of the working repos (not just the committed state, the working overlay as well).
    

The safety machine of ggo works as follows. A project has working repos. Ggo creates for each working repo a shadow repo these shadow repos act as a time machine for code changes (both commited and overlay working set changes, tags etc).

- if you make changes and if an AI makes changes to your directories a commit with a unique timestamp keeps the before and after state in a small commit.
- ggo only affects your working repos for allowed AI updates and rollbacks.
- the numerous commits in the git lfs repo and the shadow directories are navigated by time stamps and the history can be searched, e.g. if in a prompt you say "this now works (or broke)" that prompt is time stamped, can be searched with ripgrep and the state exactly before and after is available to you to restore your working repos.
    

Shadow Repo: Each project gets a parallel shadow Git repo initialized via: `git --git-dir=.ggo/ai-shadow.git --work-tree=.` The global `ai-<project-name>` (cutoff 750K bytes) is an lfs directory. Shadow directories are configured as the corresponding code directories. The user's actual `.git` repository remains completely untouched, except for stashing when the `.ggo` repo rolls back state. (NOTE: it's not 100% clear what to do with stashes). Simple pruning is not yet available as a fast forward operation because the ai- directory could be large. Maybe later.

The ai step transaction engine is not 100% atomic but practically very safe and works as follows. We start a user's ai step by anchoring it with a timestamp. If something is about to change it snapshots all working repos with that timestamp and a `<time>-before-ai` (it also includes branch information and other data, like staged files). Then ggo may introduce a change and when this finishes (successfully or aborted) ggo creates an `<time>-after-ai` tag and commit in the shadow and lfs repos. Hence here are two tags related to each ai step:

- `transaction-start-before-ai`
- `transaction-start-time-after-ai`
    

Chat records are small steps that are appended, but if included state is noticed to have changed a transaction capturing the state is again created, in the central repo. If no changes are made, we still introduce a tag on the shadow directories so that the state remains lock step. Not every transaction is fully defined yet, but in principle every ai step becomes a recorded ai record.

The `git-restore`/`gs` command can roll back and restore of working repos based on these tags. If we restore to the before tag, we get the user's state back before the transaction was started, if we use the after tag, we get the users and AI's state back. When we do this, we commit the working repo (for safety), and if it's easily possible also stash it, and again use the timestamp to find the past. I would associate a readable time tag and the state count number from the shadow directories associated with it also. If gs goes back to the head, then it can clean up its two commits and two tags.

NOTE we could roll back the ingest of state entirely, clear out put everything back, but remember, all chat prompts would also have to be sent back and forth. If the API backend does live with that, then we make the AI api nearly transactional as well. Going forward with gs is simply a matter of attaching the prompts again, going backward requires a "wipe". However, this decision is subtle. I think the real working experience is "Gemini we are building X but we broke Y in step 5. I'm rolling back the code to 5, so we start fixing Y from there again - but we try to complete X as well". That speaks against rolling the chat state back in lock step.

## 5 The ggo repl Commands

### 5.1 Default use

Starting the repl in a directory loads the project configuration chats and gains access to the tag list. The current tag is set at the head by default, but can be changed as above.
So the typical workflow is: 

- Start ggo
- Open the chat buffer
- `write-files/wf`
- work in terminal and editor as usual but be prepared for: 
	- easy creation of paste board entries in shells (for pasting into prompts)
	- having commands like: `start diff/patch session based on clipboard`

### 5.2 git and snapshot commands

- `git-list/gl <count>` git list tags with descriptions when available, without arg, all timestamps (human readable) and descs
- `git-restore/gs [tag|chat]` git set time tag or chat name (at final chat timestamp), without argument, the last one. Before executing the checkout, ggo snapshots the current working tree (edited, staged, unmodified) to prevent data loss, allowing immediate reversal.  Stashing of current state is automatic
- `git-describe/gd` git describe the tag currently set
- `git-fastforward/gc <count>` clear `<count>` tags from the current chat fast forward to last one, without arguments, fast forward to the last one. (LATER)
- `git-remote/gr [push|pull]` push/pull the lfs repo and shadow repos of the project to their upstream (described above).  The system is conflict free. 
- `git-grep/gg <pattern>` searches through the history of the selected ai records and finds the timestamp(s) when a result or error was detected.  Prints the timestamp good for selection, puts most recent one in the paste buffer.


#### 5.2.1 Ingesting

- `context-include` / `inc <path1> <path2>...`: Include. Adds a file or directory subtree to the persistent `.ggo/project.yaml` included list. Automatically ingested into the context *as read-only data*. Global documents (like Obsidian Vaults) are safely ingested without being swept into the Time Machine.
- `git-add-repo` / `gr <path>`: Explicitly enrolls a specific Git repository into the `Project.CodeDirs` state matrix, bringing it under the safety umbrella of the Time Machine.


## 6 Detailed design notes

### 6.1 The gsync Primitive: Universal State Transfer

#### 6.1.1 Concept: The Unified State Machine

To guarantee absolute safety, zero data loss, and perfect chronological alignment, ggo uses a single, bidirectional primitive called gsync. It decomposes the user's repository environment into a Foundation and an Overlay, transferring them between the Working Repo and a localized Shadow Vault.

The complete state consists of:

- Git Hash (The exact point in history)
- Branch (The symbolic reference the user was on)
- Tags (Standard human-authored release tags)
- Overlay (The uncommitted physical files on the desk)
- Staged files (Captured safely via binary patch injection)
- Untracked files (not in .gitignore)
- Stashed files (Fully preserved via `refs/stash` mirroring)
    

In the Shadow Repos, we store a perfect mirror of the Working Repos, but they also accumulate extra overlay commits representing the AI's temporal states. It is critical that these AI-generated commits never bleed back into the Working Repo's history.

#### 6.1.2 The Git Namespace Secret (refs/ggo/)

To prevent AI tags and overlay commits from polluting the user's Working Repo, ggo completely bypasses standard Git tagging. Instead, it writes transaction anchors directly into a custom, hidden namespace: `refs/ggo/txn/`. Because standard Git commands (clone, fetch, push) are hardcoded to only look at `refs/heads/` (branches) and `refs/tags/` (human tags), the AI's temporal history remains entirely invisible and perfectly isolated within the Shadow Vault.

#### 6.1.3 The Anchor Time

Every AI transaction is bound to a single Anchor Time (`YYYYMMDDTHHMMSSZ`) generated at the exact moment the prompt is submitted. This serves as the immutable Transaction ID. Because AI transactions and Git syncs are not atomic (e.g., the system could crash or lose power mid-generation), we capture three distinct anchors per transaction for crash-recovery:

- `<Anchor>-start`: Created instantly to lock the transaction.
- `<Anchor>-before`: Created after successfully capturing the user's pre-prompt desk.
- `<Anchor>-after`: Created after successfully capturing the AI's response.
    
#### 6.1.4 Direction 1: Capture (Working → Shadow)

Used to securely vault the user's state and record AI modifications.

1. **Mirror the Database**: ggo runs a raw git fetch from the Shadow Repo, pulling all branches, commits, stashes (`refs/stash`), and human tags from the Working Repo. This acts as a continuous disaster-recovery backup.
2. **Isolate the Foundation**: ggo reads the Working Repo's current Commit Hash and Branch, then checks out that exact hash inside the Shadow Repo (entering Detached HEAD to protect the mirrored branches).
3. **Extract the Index**: ggo runs a binary diff (`.ggo_staged.patch`) to safely record what files the user had explicitly staged.
4. **Capture the Overlay**: ggo executes a destructive rsync from the Working directory to the Shadow directory, perfectly capturing the uncommitted desk, and then cleans up the temporary patch file.
5. **Seal the Record**: The Shadow Repo commits the overlay—storing the Foundation metadata in the message. Instead of a normal tag, ggo runs `git update-ref refs/ggo/txn/<Anchor>-before <commit-hash>`, hiding this state safely in the custom namespace.
    

#### 6.1.5 Direction 2: Restore (Shadow → Working)

Used by the Time Machine to perfectly recreate past states without destroying human progress, and for complete Disaster Recovery. NOTE: The precise restoration mechanism requires preserving the working tree without relying on destructive resets (`git checkout -f` vs index state bugs), relying heavily on staging patches and clean overlay generation.

1. **Read the Record**: ggo queries the `refs/ggo/txn/` namespace in the Shadow Repo to locate the requested historical Foundation (Commit Hash) and Branch.
    
2. **Protect the Future**: ggo checks out that exact Commit Hash in the Working Repo (entering Detached HEAD). This guarantees that any new branches or recent human commits are left safely untouched in the future.
    
3. **Restore the Overlay**: ggo executes a targeted rsync from the Shadow directory back onto the Working desk.
    
4. **Reconstruct the Index**: ggo safely applies the `.ggo_staged.patch` if it exists, ensuring the user's staging area is mathematically restored. The user is instantly transported to the exact, mathematically perfect state of the past.
    

#### 6.1.6 Disaster Recovery (Total Restoration)

If a rogue AI command (or human error) deletes the entire Working Repo, the user simply runs `gs <tag>`. ggo detects the missing directory and executes a standard `git clone` from the Shadow Vault to restore the foundation before applying the requested overlay. Because Git ignores the `refs/ggo/` namespace by default, the clone perfectly restores the human history, branches, and tags, while automatically leaving all the AI's overlay commits behind in the vault.

### 6.2 Automatic Context Updates (The JIT Full-State Protocol)

**Concept: The Just-In-Time Full-State XML Protocol** To keep the LLM context mathematically aligned with your desk and avoid hallucination drift, `ggo` uses a Just-In-Time (JIT) protocol. Because LLMs lack internal "patch engines", feeding them stacked diffs over a long chat session severely degrades their mental model of the codebase. Instead, leveraging modern massive context windows, `ggo` detects _that_ an included file changed, but injects the _full updated text_ using a strict machine-to-machine XML schema.

This creates a rigid semantic boundary: the LLM understands everything inside `<system_context>` is the undisputed, current physical reality of the disk, and everything outside is the user's intent.

**How it Works:**

1. **The Baseline:** When you explicitly include a file (`inc src/main.go`), `ggo` copies it into the central `~/.ggo/ggoai-<project>/includes/` directory. This establishes the baseline state.
    
2. **The JIT Check:** The exact moment you submit a new chat prompt, `ggo` compares the live, physical files on your desk against their baseline copies.
    
3. **The XML Protocol Injection:** If a tracked file has been modified, `ggo` extracts its _complete current text_, wraps it in a rigid XML schema, and silently prepends it to your prompt.
    
    XML
    
    ```
    <system_context>
        <file path="src/main.go" state="modified_since_last_turn" type="full_text">
        // [The absolute, complete text of the file as it currently exists on disk]
        </file>
    </system_context>
    ```
    
1. **The New Baseline:** Immediately after the prompt is dispatched to the API, `ggo` silently copies the updated live files back to the central vault. This resets the baseline, ensuring the full text is only injected on the exact turn immediately following a local modification.

### 6.3 Chats

Git's chat pipes
- `~/.ggo/<project>/chats/<chat name>/{data-pipe, control-pipe}`
These pipes are always ready for use with an editor when this chat has been selected.  Responsibilities:
- ggo will 
	- sanitize LLM output and create ai records
	- add data like timestamps, project and chat names to the markdown
	- if a response with snippets arrives ggo places them into the paste buffers
	- if a respone with new files arrives `files-write/fw` will write the files to the working dirs (bracketed by a transaction)
	- allow a simple terminal chat with the `chat/p` command, `ctrl-d` to send, `ctrl-c` to abort. 
- the editor is responsible sending the next prompt text (search back ward to "end-response" and send a region)
- the editor is reponsible for sending control commands
	- send
	- abort
	- ANY others?


## 7 Future ideas

### 7.1 Extract to a Web Page (`ggo web`)

Running `ggo web <outfile>` will locally host a fully rendered multimedia web page of your extracted chats.

- **Asset Extraction:** Attachments (images, PDFs, CSVs) from the raw export will be copied into the `~/ai/chats/source/attachments/<chat-id>/` directory, and markdown links will be automatically rewritten to point to them.
- **HTML Rendering & Serving:** Go's `goldmark` parser will convert the markdown to styled HTML. A lightweight local server (`localhost:8080`) will spin up and automatically open the report in your default browser.
- **The "Print to PDF" Shortcut:** Instead of fighting with complex PDF generation libraries in Go, this approach offers an excellent shortcut: simply hit **Cmd+P** in your browser and select "Save as PDF" to generate a perfectly formatted, shareable document.
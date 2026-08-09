<div id="top">

<!-- HEADER STYLE: COMPACT -->
<img src="readmeai/assets/logos/ice.svg" width="30%" align="left" style="margin-right: 15px">

# <code>❯ TINY CLAW</code>
<em></em>

<!-- BADGES -->
<!-- local repository, no metadata badges. -->

<em>Built with the tools and technologies:</em>

<img src="https://img.shields.io/badge/Go-00ADD8.svg?style=flat-square&logo=Go&logoColor=white" alt="Go">

<br clear="left"/>

## 🌈 Table of Contents

<details>
<summary>Table of Contents</summary>

- [🌈 Table of Contents](#-table-of-contents)
- [🔴 Overview](#-overview)
- [🟠 Features](#-features)
- [🟡 Project Structure](#-project-structure)
    - [🟢 Project Index](#-project-index)
- [🔵 Getting Started](#-getting-started)
    - [🟣 Prerequisites](#-prerequisites)
    - [⚫ Installation](#-installation)
    - [⚪ Usage](#-usage)
    - [🟤 Testing](#-testing)
- [🌟 Roadmap](#-roadmap)
- [🤝 Contributing](#-contributing)
- [📜 License](#-license)
- [✨ Acknowledgments](#-acknowledgments)

</details>

---

## 🔴 Overview



---

## 🟠 Features

<code>❯ REPLACE-ME</code>

---

## 🟡 Project Structure

```sh
└── /
    ├── LICENSE
    ├── REMDME.md
    ├── cmd
    │   ├── claw
    │   └── larkbot
    ├── docs
    │   ├── gen.txt
    │   ├── lark-async-pipeline-design.md
    │   ├── skill-system-design.md
    │   └── superpowers
    ├── go.mod
    ├── go.sum
    ├── internal
    │   ├── config
    │   ├── context
    │   ├── engine
    │   ├── gateway
    │   ├── provider
    │   ├── schema
    │   ├── tookit
    │   └── tools
    └── readme-ai.md
```

### 🟢 Project Index

<details open>
	<summary><b><code>/</code></b></summary>
	<!-- __root__ Submodule -->
	<details>
		<summary><b>__root__</b></summary>
		<blockquote>
			<div class='directory-path' style='padding: 8px 0; color: #666;'>
				<code><b>⦿ __root__</b></code>
			<table style='width: 100%; border-collapse: collapse;'>
			<thead>
				<tr style='background-color: #f8f9fa;'>
					<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
					<th style='text-align: left; padding: 8px;'>Summary</th>
				</tr>
			</thead>
				<tr style='border-bottom: 1px solid #eee;'>
					<td style='padding: 8px;'><b><a href='/go.mod'>go.mod</a></b></td>
					<td style='padding: 8px;'>- Defines the Go module for tiny-claw, establishing the projects identity and dependency graph<br>- It pins the Go runtime version and declares direct dependencies on Anthropics Claude SDK, OpenAI's API client, and Larksuite's SDK, enabling multi-provider AI integration and enterprise messaging<br>- Indirect dependencies support JSON schema validation, websockets, and YAML processing, forming the foundational build configuration.</td>
				</tr>
				<tr style='border-bottom: 1px solid #eee;'>
					<td style='padding: 8px;'><b><a href='/LICENSE'>LICENSE</a></b></td>
					<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
				</tr>
				<tr style='border-bottom: 1px solid #eee;'>
					<td style='padding: 8px;'><b><a href='/go.sum'>go.sum</a></b></td>
					<td style='padding: 8px;'>- The <code>go.sum</code> file serves as a trust anchor for the Go module dependency graph<br>- It pins the exact cryptographic checksums of every third‑party module and its transitive dependencies (e.g., <code>anthropics/anthropic-sdk-go</code>), guaranteeing that builds are reproducible and that no tampered or corrupted module version can slip into the project<br>- By validating module integrity against these recorded hashes, it ensures the supply chain remains consistent and secure across different environments and over time.</td>
				</tr>
			</table>
		</blockquote>
	</details>
	<!-- internal Submodule -->
	<details>
		<summary><b>internal</b></summary>
		<blockquote>
			<div class='directory-path' style='padding: 8px 0; color: #666;'>
				<code><b>⦿ internal</b></code>
			<!-- tools Submodule -->
			<details>
				<summary><b>tools</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ internal.tools</b></code>
					<table style='width: 100%; border-collapse: collapse;'>
					<thead>
						<tr style='background-color: #f8f9fa;'>
							<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
							<th style='text-align: left; padding: 8px;'>Summary</th>
						</tr>
					</thead>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/skill_tool.go'>skill_tool.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/skill_tool_test.go'>skill_tool_test.go</a></b></td>
							<td style='padding: 8px;'>- Validates the SkillTool component by checking its name, definition schema, and execution output<br>- Ensures skill bodies are correctly prefixed with compliance instructions and truncated at 32KB with a marker<br>- Provides compile-time assertion that SkillTool conforms to the BaseTool interface, reinforcing tool registration reliability.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/read_file_test.go'>read_file_test.go</a></b></td>
							<td style='padding: 8px;'>Properly responding to agent requests, returning file contents or appropriate errors, and respecting the security boundary of the working directory.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/files.go'>files.go</a></b></td>
							<td style='padding: 8px;'>- Ensures safe file operations within a constrained workspace by validating relative paths, preventing directory escapes, and rejecting oversized or non-regular files<br>- Atomic writes via temporary files and rename guarantee readers never observe partial content, while context cancellation is honored up to the commit point<br>- These primitives form the foundation for tools that read and write workspace files securely.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/parallel_test.go'>parallel_test.go</a></b></td>
							<td style='padding: 8px;'>- Validates concurrent safety of file operation tools within the broader architecture<br>- These tests confirm parallel edits preserve every update, mixed readers and writers never observe corrupted content, and independent file writes remain isolated<br>- They exercise the packages synchronization guarantees, ensuring the tools remain reliable when multiple clients modify workspaces simultaneously.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/edit_file.go'>edit_file.go</a></b></td>
							<td style='padding: 8px;'>Provides the edit_file tool capability within the agent toolkit, enabling safe, targeted modification</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/bash.go'>bash.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/registry.go'>registry.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/write_file_test.go'>write_file_test.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/filelock_test.go'>filelock_test.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/filelock.go'>filelock.go</a></b></td>
							<td style='padding: 8px;'>- Coordinates filesystem tool concurrency by providing per-path read-write locks, allowing disjoint operations to proceed in parallel while serializing same-path accesses<br>- A global exclusive tier reserves bash commands that may touch any file<br>- Shared process-wide rather than per-tool, it guarantees synchronization across separate</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/write_file.go'>write_file.go</a></b></td>
							<td style='padding: 8px;'>- Enables the agent to create or overwrite files within the workspace from a relative path and content, auto-creating parent directories<br>- Enforces confinement against traversal and symlink escapes, caps oversized payloads, and serializes writes to the same destination while permitting concurrent writes to distinct paths.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/bash_test.go'>bash_test.go</a></b></td>
							<td style='padding: 8px;'>Validates the Bash tool’s integration within the agent tool registry, covering command output capture, working-directory execution, stderr merging, non-zero exit codes as data, empty and invalid argument errors, output truncation, configurable timeouts, and context cancellation, ensuring reliable shell execution and consistent error propagation across the architecture.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/edit_file_test.go'>edit_file_test.go</a></b></td>
							<td style='padding: 8px;'>- Validates the edit_file tools core behaviors within the tools package, covering exact and multi-line replacements, unique-match enforcement, fuzzy indentation matching, and error handling for missing or empty text<br>- Security tests confirm rejection of path traversal and symlink escapes, while cancellation tests ensure files remain unmodified when context is canceled<br>- Together these verify the tools safety, correctness, and integration readiness.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tools/read_file.go'>read_file.go</a></b></td>
							<td style='padding: 8px;'>- Provides a workspace-safe file reading tool within the agents tool execution pipeline<br>- It retrieves file contents using relative paths, enforces strict workspace containment, blocks symlink escapes, caps output length, truncates oversized text with head and tail excerpts, and substitutes binary files with compact metadata summaries<br>- Coordinates concurrent access through path-level locking.</td>
						</tr>
					</table>
				</blockquote>
			</details>
			<!-- context Submodule -->
			<details>
				<summary><b>context</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ internal.context</b></code>
					<table style='width: 100%; border-collapse: collapse;'>
					<thead>
						<tr style='background-color: #f8f9fa;'>
							<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
							<th style='text-align: left; padding: 8px;'>Summary</th>
						</tr>
					</thead>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/context/skill.go'>skill.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/context/composer.go'>composer.go</a></b></td>
							<td style='padding: 8px;'>- Builds the system prompt that defines tiny-claws assistant behavior by composing core identity rules, optional plan-mode persistence mandates, project-specific AGENT.md guidelines, and skill usage strategy<br>- It dynamically adapts the prompt based on whether long-running task mode is active and whether the workspace provides custom architecture guidance, returning a complete system message for the conversation.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/context/skill_register.go'>skill_register.go</a></b></td>
							<td style='padding: 8px;'>- Registers all skills discovered under a working directory into the central tool registry<br>- Loads skill definitions, normalizes their names to ensure valid tool identifiers, and registers each as an executable tool<br>- Individual registration failures are logged as warnings and skipped without aborting the process, while an empty skill set completes successfully.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/context/composer_test.go'>composer_test.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/context/skill_test.go'>skill_test.go</a></b></td>
							<td style='padding: 8px;'>- Validates skill loading and frontmatter parsing for SKILL.md files stored under.claw/skills directories<br>- Ensures correct extraction of skill name, description, and body content from standard, quoted, and colon-containing frontmatter<br>- Confirms graceful fallback defaults when frontmatter is absent, proper error on malformed delimiters, and resilient directory loading that skips invalid files while retaining valid skills.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/context/skill_register_test.go'>skill_register_test.go</a></b></td>
							<td style='padding: 8px;'>Validates the skill registration pipeline for the tool registry, confirming that skill names</td>
						</tr>
					</table>
				</blockquote>
			</details>
			<!-- config Submodule -->
			<details>
				<summary><b>config</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ internal.config</b></code>
					<table style='width: 100%; border-collapse: collapse;'>
					<thead>
						<tr style='background-color: #f8f9fa;'>
							<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
							<th style='text-align: left; padding: 8px;'>Summary</th>
						</tr>
					</thead>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/config/config.go'>config.go</a></b></td>
							<td style='padding: 8px;'>- Manages centralized configuration loading by layering defaults, global, project, and local settings files, then applying environment variable overrides with highest precedence<br>- Ensures required model and API key fields are validated before initializing logging<br>- Supports CLI and Lark bot modes through unified settings structures compatible with Claude Code conventions.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/config/slog_setup.go'>slog_setup.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/config/config_test.go'>config_test.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
					</table>
				</blockquote>
			</details>
			<!-- provider Submodule -->
			<details>
				<summary><b>provider</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ internal.provider</b></code>
					<table style='width: 100%; border-collapse: collapse;'>
					<thead>
						<tr style='background-color: #f8f9fa;'>
							<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
							<th style='text-align: left; padding: 8px;'>Summary</th>
						</tr>
					</thead>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/provider/provider_test.go'>provider_test.go</a></b></td>
							<td style='padding: 8px;'>- Verifies provider selection logic routes correctly to OpenAI or Claude implementations based on configuration, ensures unknown providers return errors, and confirms missing API keys are rejected<br>- This safeguards the architecture’s extensibility and fail-fast behavior across different LLM backends<br>- Succinctly covers the critical contract for provider initialization.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/provider/interface.go'>interface.go</a></b></td>
							<td style='padding: 8px;'>- Defines a unified interface for language model providers and a factory that routes to OpenAI or Claude implementations based on configuration<br>- Defaulting to OpenAI when unspecified, it abstracts provider differences so the rest of the system can request completions with tools uniformly, simplifying integration and enabling future provider additions.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/provider/claude.go'>claude.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/provider/openai.go'>openai.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
					</table>
				</blockquote>
			</details>
			<!-- tookit Submodule -->
			<details>
				<summary><b>tookit</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ internal.tookit</b></code>
					<table style='width: 100%; border-collapse: collapse;'>
					<thead>
						<tr style='background-color: #f8f9fa;'>
							<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
							<th style='text-align: left; padding: 8px;'>Summary</th>
						</tr>
					</thead>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tookit/json_test.go'>json_test.go</a></b></td>
							<td style='padding: 8px;'>- Verifies the toolkits JSON conversion and line-append utilities, ensuring Any2Json correctly serializes structs for round-trip decoding and AppendLine automatically creates missing directories while appending formatted lines<br>- Tests also confirm proper error propagation when parent paths are files or targets are directories, safeguarding logging and data-persistence workflows.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/tookit/json.go'>json.go</a></b></td>
							<td style='padding: 8px;'>- Provides shared utility functions for serializing arbitrary data into JSON strings and appending lines to files with automatic directory creation<br>- These helpers support logging, configuration output, and persistent storage across the codebase, enabling consistent data transformation and file writing without repetitive boilerplate<br>- Also automates standard file permissions and ensures errors are handled gracefully throughout.</td>
						</tr>
					</table>
				</blockquote>
			</details>
			<!-- schema Submodule -->
			<details>
				<summary><b>schema</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ internal.schema</b></code>
					<table style='width: 100%; border-collapse: collapse;'>
					<thead>
						<tr style='background-color: #f8f9fa;'>
							<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
							<th style='text-align: left; padding: 8px;'>Summary</th>
						</tr>
					</thead>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/schema/message.go'>message.go</a></b></td>
							<td style='padding: 8px;'>- Defines the core data contracts for AI conversation messages, tool invocations, and tool results<br>- These structures standardize how user, system, and assistant messages are serialized, while capturing tool call metadata and results<br>- They enable seamless interoperability between the agent runtime and external tools, ensuring consistent request and response formatting throughout the system.</td>
						</tr>
					</table>
				</blockquote>
			</details>
			<!-- engine Submodule -->
			<details>
				<summary><b>engine</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ internal.engine</b></code>
					<table style='width: 100%; border-collapse: collapse;'>
					<thead>
						<tr style='background-color: #f8f9fa;'>
							<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
							<th style='text-align: left; padding: 8px;'>Summary</th>
						</tr>
					</thead>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/nop_reporter.go'>nop_reporter.go</a></b></td>
							<td style='padding: 8px;'>- NopReporter provides a silent implementation of the Reporter interface, allowing the engine to function in CLI mode or other scenarios where progress callbacks are unnecessary<br>- It consumes all reporting events—thinking, tool calls, tool results, and messages—without acting on them, preserving engine compatibility while maintaining clean output.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/session.go'>session.go</a></b></td>
							<td style='padding: 8px;'>- Manages LLM conversation sessions by tracking dialog history, estimating token usage, and compressing old messages into summaries when context windows approach capacity<br>- Persists session state to disk and reconstructs sessions on load<br>- Supports configurable context windows, compression ratios, and custom summarizers while ensuring boundaries between conversation turns remain intact during truncation.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/reporter.go'>reporter.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/terminal_reporter.go'>terminal_reporter.go</a></b></td>
							<td style='padding: 8px;'>- Implements the terminal-based Reporter interface for the engine, surfacing agent activity directly in the console<br>- It prints thinking indicators, tool invocation notices, and assistant messages using emoji markers, giving users real-time visibility into the agents workflow<br>- NewTerminalReporter supplies this lightweight observer to the engine, enabling transparent progress tracking without cluttering the core execution logic.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/summarizer.go'>summarizer.go</a></b></td>
							<td style='padding: 8px;'>- Implements an LLM-driven conversation summarizer integral to the session history compression pipeline<br>- It transforms an existing summary alongside newly accumulated chat messages into a streamlined prompt, invokes the configured language model provider to produce a refreshed Chinese summary, and hands it back for context pruning<br>- Provider failures propagate upward, enabling the session layer to gracefully fall back to plain truncation.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/loop.go'>loop.go</a></b></td>
							<td style='padding: 8px;'>- Manages the core agentic execution loop, orchestrating LLM generation, tool selection, and parallel tool execution<br>- Coordinates multi-turn session memory so successful rounds persist while failed ones do not<br>- Routes progress updates through the reporter for user visibility, loops until the model stops requesting tools, and respects the configured work directory settings.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/loop_test.go'>loop_test.go</a></b></td>
							<td style='padding: 8px;'>- Validates the agent engines core conversational loop through tests covering stateless operation, session persistence, multi-turn memory, and tool execution cycles<br>- Confirms messages are saved and restored correctly across restarts, tool results are included in history, and failed turns leave no trace<br>- Ensures the engine maintains coherent context while handling errors gracefully.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/session_test.go'>session_test.go</a></b></td>
							<td style='padding: 8px;'>- This test file verifies the core session-handling behavior of the engine layer<br>- It ensures that conversation state—including message roles, tool results, and token estimates—is assembled correctly so that the engine can reliably manage multi-turn interactions<br>- By validating these session mechanics, it protects the integrity of the overall request-processing pipeline.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/summarizer_test.go'>summarizer_test.go</a></b></td>
							<td style='padding: 8px;'>- Validates the LLM summarizer’s prompt construction by verifying system and context messages, inclusion of existing summaries, proper formatting of user, assistant, tool calls, and results, skipping empty summaries, and propagating provider errors<br>- Ensures summarization behavior aligns with engine expectations.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/session_message.go'>session_message.go</a></b></td>
							<td style='padding: 8px;'>- Manages the complete lifecycle of chat sessions within the engine, serving as a concurrency-safe registry<br>- It retrieves existing sessions or restores them from disk after restarts, enables reset semantics by deleting session data, and provides graceful shutdown persistence<br>- This central manager ensures multi-turn conversations remain available across process restarts.</td>
						</tr>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/internal/engine/session_message_test.go'>session_message_test.go</a></b></td>
							<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
						</tr>
					</table>
				</blockquote>
			</details>
			<!-- gateway Submodule -->
			<details>
				<summary><b>gateway</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ internal.gateway</b></code>
					<!-- lark Submodule -->
					<details>
						<summary><b>lark</b></summary>
						<blockquote>
							<div class='directory-path' style='padding: 8px 0; color: #666;'>
								<code><b>⦿ internal.gateway.lark</b></code>
							<table style='width: 100%; border-collapse: collapse;'>
							<thead>
								<tr style='background-color: #f8f9fa;'>
									<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
									<th style='text-align: left; padding: 8px;'>Summary</th>
								</tr>
							</thead>
								<tr style='border-bottom: 1px solid #eee;'>
									<td style='padding: 8px;'><b><a href='/internal/gateway/lark/worker.go'>worker.go</a></b></td>
									<td style='padding: 8px;'>- Defines a serial message-processing worker for the Lark gateway, consuming incoming messages one at a time to preserve ordering<br>- Each message passes through an injected processor, with panic recovery and error notification ensuring the worker remains resilient<br>- The worker shuts down gracefully on context cancellation, completing the current message before stopping.</td>
								</tr>
								<tr style='border-bottom: 1px solid #eee;'>
									<td style='padding: 8px;'><b><a href='/internal/gateway/lark/queue.go'>queue.go</a></b></td>
									<td style='padding: 8px;'>- Provides a deduplication layer and bounded message queue for the Lark gateway, ensuring each msg_id is processed only once within a configurable TTL window while preventing blocking on full queues<br>- Repeated deliveries from WebSocket reconnections are rejected as duplicates, and overflow messages are silently dropped to apply backpressure, maintaining stable message flow and bounded memory usage.</td>
								</tr>
								<tr style='border-bottom: 1px solid #eee;'>
									<td style='padding: 8px;'><b><a href='/internal/gateway/lark/message.go'>message.go</a></b></td>
									<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
								</tr>
								<tr style='border-bottom: 1px solid #eee;'>
									<td style='padding: 8px;'><b><a href='/internal/gateway/lark/queue_test.go'>queue_test.go</a></b></td>
									<td style='padding: 8px;'>- Validates the Lark gateways message queue and deduplication mechanisms<br>- Tests confirm bounded queues accept messages until full, then reject new ones without blocking<br>- Duplicate message IDs are discarded within the dedupers TTL window, while empty IDs pass through<br>- Also verifies TTL expiry resets deduplication state, ensuring reliable inbound message processing.</td>
								</tr>
								<tr style='border-bottom: 1px solid #eee;'>
									<td style='padding: 8px;'><b><a href='/internal/gateway/lark/lark.go'>lark.go</a></b></td>
									<td style='padding: 8px;'>- Manages Lark bot connectivity by establishing a WebSocket long connection for receiving events and sending text messages to chats<br>- It abstracts the Lark SDK behind a Bot type, handling event registration, connection lifecycle, and message delivery with tenant support, enabling the gateway layer to integrate Lark messaging without hardcoded credentials.</td>
								</tr>
								<tr style='border-bottom: 1px solid #eee;'>
									<td style='padding: 8px;'><b><a href='/internal/gateway/lark/message_test.go'>message_test.go</a></b></td>
									<td style='padding: 8px;'>- Validates the Lark message parsing logic by exercising ParseMessageEvent across representative scenarios<br>- Ensures user text messages are correctly extracted with identifiers and content, while bot-originated, non-text, malformed, nil, or incomplete events are properly rejected<br>- Confirms gateway reliability when normalizing incoming Lark webhook payloads for downstream processing.</td>
								</tr>
								<tr style='border-bottom: 1px solid #eee;'>
									<td style='padding: 8px;'><b><a href='/internal/gateway/lark/lark_reporter.go'>lark_reporter.go</a></b></td>
									<td style='padding: 8px;'>Implements the engine.Reporter interface to stream real-time progress updates into a designated Lark chat, translating engine lifecycle callbacks—thinking, tool calls, tool results, and messages—into friendly user</td>
								</tr>
								<tr style='border-bottom: 1px solid #eee;'>
									<td style='padding: 8px;'><b><a href='/internal/gateway/lark/engine_processor.go'>engine_processor.go</a></b></td>
									<td style='padding: 8px;'>Code>❯ REPLACE-ME</code></td>
								</tr>
								<tr style='border-bottom: 1px solid #eee;'>
									<td style='padding: 8px;'><b><a href='/internal/gateway/lark/worker_test.go'>worker_test.go</a></b></td>
									<td style='padding: 8px;'>- Validates the lark gateway worker’s core reliability by exercising ordered message consumption, panic recovery that keeps the worker alive, error callback propagation, and graceful shutdown upon context cancellation<br>- These tests confirm resilient serial processing, proper failure notification, and clean lifecycle behavior within the gateway’s message-handling architecture.</td>
								</tr>
							</table>
						</blockquote>
					</details>
				</blockquote>
			</details>
		</blockquote>
	</details>
	<!-- cmd Submodule -->
	<details>
		<summary><b>cmd</b></summary>
		<blockquote>
			<div class='directory-path' style='padding: 8px 0; color: #666;'>
				<code><b>⦿ cmd</b></code>
			<!-- larkbot Submodule -->
			<details>
				<summary><b>larkbot</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ cmd.larkbot</b></code>
					<table style='width: 100%; border-collapse: collapse;'>
					<thead>
						<tr style='background-color: #f8f9fa;'>
							<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
							<th style='text-align: left; padding: 8px;'>Summary</th>
						</tr>
					</thead>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/cmd/larkbot/main.go'>main.go</a></b></td>
							<td style='padding: 8px;'>Loads configuration, validates Lark credentials, initializes the AI provider and tool registry with file, edit, and bash tools, registers skills, then starts a WebSocket listener where events are parsed and enqueued for a single worker to handle asynchronously, exiting gracefully on SIGINT/SIGTERM.</td>
						</tr>
					</table>
				</blockquote>
			</details>
			<!-- claw Submodule -->
			<details>
				<summary><b>claw</b></summary>
				<blockquote>
					<div class='directory-path' style='padding: 8px 0; color: #666;'>
						<code><b>⦿ cmd.claw</b></code>
					<table style='width: 100%; border-collapse: collapse;'>
					<thead>
						<tr style='background-color: #f8f9fa;'>
							<th style='width: 30%; text-align: left; padding: 8px;'>File Name</th>
							<th style='text-align: left; padding: 8px;'>Summary</th>
						</tr>
					</thead>
						<tr style='border-bottom: 1px solid #eee;'>
							<td style='padding: 8px;'><b><a href='/cmd/claw/main.go'>main.go</a></b></td>
							<td style='padding: 8px;'>- Loads configuration, initializes the AI provider, registers file manipulation and shell tools alongside skills, then constructs the agent engine and executes the user-provided prompt<br>- Each initialization step validates its outcome, logging errors and exiting with code 1 on any failure.</td>
						</tr>
					</table>
				</blockquote>
			</details>
		</blockquote>
	</details>
</details>

---

## 🔵 Getting Started

### 🟣 Prerequisites

This project requires the following dependencies:

- **Programming Language:** Go
- **Package Manager:** Go modules

### ⚫ Installation

Build  from the source and intsall dependencies:

1. **Clone the repository:**

    ```sh
    ❯ git clone ../
    ```

2. **Navigate to the project directory:**

    ```sh
    ❯ cd 
    ```

3. **Install the dependencies:**

<!-- SHIELDS BADGE CURRENTLY DISABLED -->
	<!-- [![go modules][go modules-shield]][go modules-link] -->
	<!-- REFERENCE LINKS -->
	<!-- [go modules-shield]: https://img.shields.io/badge/Go-00ADD8.svg?style={badge_style}&logo=go&logoColor=white -->
	<!-- [go modules-link]: https://golang.org/ -->

	**Using [go modules](https://golang.org/):**

	```sh
	❯ go build
	```

### ⚪ Usage

Run the project with:

**Using [go modules](https://golang.org/):**
```sh
go run {entrypoint}
```

### 🟤 Testing

 uses the {__test_framework__} test framework. Run the test suite with:

**Using [go modules](https://golang.org/):**
```sh
go test ./...
```

---

## 🌟 Roadmap

- [X] **`Task 1`**: <strike>Implement feature one.</strike>
- [ ] **`Task 2`**: Implement feature two.
- [ ] **`Task 3`**: Implement feature three.

---

## 🤝 Contributing

- **💬 [Join the Discussions](https://LOCAL///discussions)**: Share your insights, provide feedback, or ask questions.
- **🐛 [Report Issues](https://LOCAL///issues)**: Submit bugs found or log feature requests for the `` project.
- **💡 [Submit Pull Requests](https://LOCAL///blob/main/CONTRIBUTING.md)**: Review open PRs, and submit your own PRs.

<details closed>
<summary>Contributing Guidelines</summary>

1. **Fork the Repository**: Start by forking the project repository to your LOCAL account.
2. **Clone Locally**: Clone the forked repository to your local machine using a git client.
   ```sh
   git clone .
   ```
3. **Create a New Branch**: Always work on a new branch, giving it a descriptive name.
   ```sh
   git checkout -b new-feature-x
   ```
4. **Make Your Changes**: Develop and test your changes locally.
5. **Commit Your Changes**: Commit with a clear message describing your updates.
   ```sh
   git commit -m 'Implemented new feature x.'
   ```
6. **Push to LOCAL**: Push the changes to your forked repository.
   ```sh
   git push origin new-feature-x
   ```
7. **Submit a Pull Request**: Create a PR against the original project repository. Clearly describe the changes and their motivations.
8. **Review**: Once your PR is reviewed and approved, it will be merged into the main branch. Congratulations on your contribution!
</details>

<details closed>
<summary>Contributor Graph</summary>
<br>
<p align="left">
   <a href="https://LOCAL{///}graphs/contributors">
      <img src="https://contrib.rocks/image?repo=/">
   </a>
</p>
</details>

---

## 📜 License

 is protected under the [LICENSE](https://choosealicense.com/licenses) License. For more details, refer to the [LICENSE](https://choosealicense.com/licenses/) file.

---

## ✨ Acknowledgments

- Credit `contributors`, `inspiration`, `references`, etc.

<div align="right">

[![][back-to-top]](#top)

</div>


[back-to-top]: https://img.shields.io/badge/-BACK_TO_TOP-151515?style=flat-square


---

# nAI

Ask questions about the files on your NAS and get answers that point to the exact file and page.
nAI runs on your own hardware; nothing leaves your network unless you configure a cloud model.

## Features

- Indexes NithronOS shares in place (read-only) and keeps the index in sync as files change
- Finds documents by meaning, in many languages ("car insurance contract" finds `scan_0042.pdf`)
- Reads PDF, Word, Markdown, HTML, text, data and code files; OCR for scans and photos of documents
- Answers with citations (file, page, section) that open the original file
- Per-group access: a share can be visible only to some groups
- Works without a GPU; generated answers can use Ollama on the NAS or on a PC with a GPU

## Initial Setup

1. Open nAI (default http://<nas>:8090, or https://<nas>/apps/nai/)
2. Create the first account; it becomes the nAI admin
3. Open **Sources** and add shares, e.g. name `Documents`, path `/shares/documents`
4. Indexing starts right away. The search model (~0.5 GB) downloads on first start; until it is
   ready, search matches keywords only

## Generated answers (optional)

Without a language model, nAI answers with exact quotes from your files. To get written answers:

- **Ollama on the NAS**: install Ollama, `ollama pull qwen3:4b`, set *Generated Answers* to `true`
- **Ollama on a PC with a GPU**: set *Model Server URL* to `http://<pc-ip>:11434`
- **Other servers**: provider `openai` + URL for vLLM, llama.cpp or LM Studio; or a cloud API key
  (questions and matching passages are then sent to that provider)

With model `auto`, nAI picks the largest installed Ollama model that fits the hardware.

## Permissions

nAI runs as `PUID:PGID` (default 1000:1000). That user needs read access to the shares you add
and write access to `/srv/apps/nai/data`. Files it cannot read are skipped and logged.

## Resources

About 1–1.5 GB of RAM (embedding model, reranker, OCR). The default limit is 3 GB.

# Reading document formats (PDF, DOCX)

The `read` tool extracts plain text directly from `.pdf` and `.docx`
files — pass the file path exactly like you would for a `.txt`/`.go`/
`.md` file, no special handling needed on your end. You don't need to
ask the user to convert the file first.

**What you get back:** plain text content, in reading order for DOCX
(paragraph by paragraph) and in the PDF's internal text order (which can
occasionally differ from visual order for complex multi-column layouts).

**What's lost in extraction:** formatting (bold/italic/font), tables
(DOCX tables aren't extracted — only paragraph body text), images, and
headers/footers. If the user needs table data specifically from a DOCX
and the extracted text looks wrong/missing for that part, say so plainly
rather than guessing at numbers that aren't actually in the extracted
text.

**Scanned PDFs** (a PDF that's really just images of pages, common for
scanned paper documents) have no extractable text layer — the read
tool will report an error like "no extractable text" rather than
silently returning nothing. There's no OCR capability available — tell
the user this directly rather than fabricating what the document might
say.

**Exporting documents:** first use `write` to save the report as Markdown,
then call `export_document(source, path)` with a new `.pdf` or `.docx`
output path. The exporter runs local Pandoc; PDF also needs XeLaTeX or the
engine selected by `OFFCODE_PDF_ENGINE`. If a dependency is missing, report
the error and keep the Markdown source. Never write plain text under a
`.pdf` or `.docx` extension. The exporter preserves existing output files.

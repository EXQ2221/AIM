from __future__ import annotations

import io
import unittest
from zipfile import ZipFile

from app.parser_core import parse_document


class ParseDocumentStreamTests(unittest.TestCase):
    def test_parse_text_from_stream(self) -> None:
        stream = io.BytesIO(b"\xef\xbb\xbfhello\nworld\n")

        doc = parse_document("notes.txt", "text/plain", stream)

        self.assertEqual(doc.source_type, "TEXT")
        self.assertEqual(doc.content, "hello\nworld")

    def test_parse_docx_from_stream(self) -> None:
        stream = io.BytesIO()
        with ZipFile(stream, "w") as zf:
            xml_data = """<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>hello</w:t></w:r></w:p><w:p><w:r><w:t>world</w:t></w:r></w:p></w:body></w:document>"""
            zf.writestr("word/document.xml", xml_data)
        stream.seek(0)

        doc = parse_document(
            "notes.docx",
            "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
            stream,
        )

        self.assertEqual(doc.source_type, "TEXT")
        self.assertEqual(doc.content, "hello\n\nworld")


if __name__ == "__main__":
    unittest.main()

# PDF compression fixture

`uncompressed.pdf` is a synthetic three-page A4 report generated with ReportLab 5.0.0.
It contains 32 scenario lines per page, a unique page sentinel, and deliberately
redundant whitespace in each uncompressed content stream. It contains no real run data.

The integration tests exercise real Ghostscript compression with all three presets,
check all page sentinels and end-of-page scenario text, verify private permissions,
and confirm that the source stays unchanged. Separate helper-process tests cover
larger-output fallback, changed page counts, process failure and cancellation.
This fixture tests deterministic redundancy reduction; it is not a prediction of
compression ratios for arbitrary customer reports.

`owner-restricted.pdf` is a synthetic encrypted copy with an empty user password and
a test owner password. It verifies refusal of protection-removing compression, even
when the PDF can be opened without asking for a password. It contains no private data.

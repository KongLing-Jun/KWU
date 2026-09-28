// Run: tsx src/__tests__/attachment-display.test.ts

import { bareGeneratedFilePathsForDisplay, baseName, formatAttachmentRefForDisplay, formatAttachmentRefForSubmit, linkVerifiedBareGeneratedFilesForDisplay, localFilePathFromHref, parseAttachmentRefsForDisplay, parseGeneratedFileRefsForDisplay, replaceAttachmentRefsForDisplay, restoreAttachmentRefsForSubmit, sortDisplayAttachments } from "../lib/attachmentDisplay";

let passed = 0;
let failed = 0;

function eq(a: unknown, b: unknown, label: string) {
  if (JSON.stringify(a) === JSON.stringify(b)) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

console.log("\nattachment display");

eq(
  localFilePathFromHref("%E5%AD%A3%E5%BA%A6%E7%BB%8F%E8%90%A5%E8%AF%8A%E6%96%AD%E4%B8%8E%E5%86%B3%E7%AD%96%E6%96%B9%E6%A1%88.xlsx"),
  "季度经营诊断与决策方案.xlsx",
  "decodes URL-encoded Chinese local filenames",
);

eq(
  localFilePathFromHref("reports/Q1%20report%20%23100%25.xlsx"),
  "reports/Q1 report #100%.xlsx",
  "decodes spaces, hash signs, and percent signs in local filenames",
);

eq(
  localFilePathFromHref("Q1%20report%20100%.xlsx"),
  "Q1 report 100%.xlsx",
  "decodes valid escapes while preserving a stray percent sign",
);

eq(
  localFilePathFromHref("file:///D:/reports/Q1%20report%20100%.xlsx"),
  "D:/reports/Q1 report 100%.xlsx",
  "file URLs reuse tolerant local-path percent decoding",
);

eq(
  parseAttachmentRefsForDisplay("@D:/reports/quarter.xlsx @D:\\reports\\quarter.xlsx @D:/reports/quarter.xlsx").attachments.length,
  1,
  "deduplicates repeated attachment paths across Windows slash styles",
);

eq(
  parseGeneratedFileRefsForDisplay("@D:/reports/quarter.xlsx [quarter.xlsx](D:\\reports\\quarter.xlsx)").attachments.length,
  1,
  "deduplicates generated file references across reference formats",
);

eq(
  linkVerifiedBareGeneratedFilesForDisplay("[report.xlsx](reports/report.xlsx)", new Set(["report.xlsx", "reports/report.xlsx"])),
  "[report.xlsx](reports/report.xlsx)",
  "does not nest links around filenames already inside Markdown links",
);

eq(
  linkVerifiedBareGeneratedFilesForDisplay(
    '[report.xlsx](reports/report.xlsx "download")',
    new Set(["report.xlsx", "reports/report.xlsx"]),
  ),
  '[report.xlsx](reports/report.xlsx "download")',
  "preserves Markdown links with titles",
);

eq(
  linkVerifiedBareGeneratedFilesForDisplay(
    '[report.xlsx][download]\n\n[download]: reports/report.xlsx "download"',
    new Set(["report.xlsx", "reports/report.xlsx"]),
  ),
  '[report.xlsx][download]\n\n[download]: reports/report.xlsx "download"',
  "preserves reference-style Markdown links and definitions",
);

eq(
  linkVerifiedBareGeneratedFilesForDisplay(
    "```text\nreport.xlsx\n```\nreport.xlsx",
    new Set(["report.xlsx"]),
  ),
  "```text\nreport.xlsx\n```\n[report.xlsx](<report.xlsx>)",
  "links plain filenames but preserves fenced code blocks",
);

eq(
  linkVerifiedBareGeneratedFilesForDisplay(
    "<span>report.xlsx</span>\nreport.xlsx",
    new Set(["report.xlsx"]),
  ),
  "<span>report.xlsx</span>\n[report.xlsx](<report.xlsx>)",
  "links plain filenames but preserves raw HTML content",
);

for (const [markdown, label] of [
  ['[outer [x]](reports/report.xlsx)', "nested link labels"],
  ['[download](reports/report "see ) report.xlsx")', "closing parentheses inside link titles"],
  ['[download]:\n  reports/target\n  "report.xlsx"', "multiline reference definitions"],
  ['<img\n  src="report.xlsx"\n  alt="x"\n>', "multiline raw HTML"],
] as const) {
  eq(
    linkVerifiedBareGeneratedFilesForDisplay(markdown, new Set(["report.xlsx", "reports/report.xlsx"])),
    markdown,
    `preserves ${label}`,
  );
}

eq(
  linkVerifiedBareGeneratedFilesForDisplay("- output:\n    report.xlsx", new Set(["report.xlsx"])),
  "- output:\n    [report.xlsx](<report.xlsx>)",
  "links filenames in indented list continuation text",
);

eq(
  linkVerifiedBareGeneratedFilesForDisplay("\\` report.xlsx \\`", new Set(["report.xlsx"])),
  "\\` [report.xlsx](<report.xlsx>) \\`",
  "links filenames between escaped backticks",
);

eq(
  bareGeneratedFilePathsForDisplay("已生成 Q1 report.xlsx"),
  ["已生成 Q1 report.xlsx", "Q1 report.xlsx", "report.xlsx"],
  "collects suffix candidates for unquoted filenames containing spaces",
);

eq(
  linkVerifiedBareGeneratedFilesForDisplay("已生成 Q1 report.xlsx", new Set(["Q1 report.xlsx"])),
  "已生成 [Q1 report.xlsx](<Q1 report.xlsx>)",
  "links the longest verified filename candidate containing spaces",
);

const named = parseAttachmentRefsForDisplay(
  "review @[DS30000.sl2](.kwu/attachments/clipboard-20260610-121238.444775-000002.sl2) and @[park.png](.kwu/attachments/clipboard-20260610-121238.444775-000001.png)",
);

eq(named.text, "review and", "removes named display refs from message text");
eq(
  named.attachments.map((a) => ({ path: a.path, name: a.name, kind: a.kind, ext: a.ext })),
  [
    {
      path: ".kwu/attachments/clipboard-20260610-121238.444775-000002.sl2",
      name: "DS30000.sl2",
      kind: "file",
      ext: "SL2",
    },
    {
      path: ".kwu/attachments/clipboard-20260610-121238.444775-000001.png",
      name: "park.png",
      kind: "image",
      ext: "PNG",
    },
  ],
  "preserves original display names for attachment cards",
);
eq(
  sortDisplayAttachments(named.attachments).map((a) => a.name),
  ["park.png", "DS30000.sl2"],
  "sorts images before files while keeping groups stable",
);
eq(
  replaceAttachmentRefsForDisplay("see @[DS30000.sl2](.kwu/attachments/clipboard-20260610-121238.444775-000002.sl2)"),
  "see [file:DS30000.sl2]",
  "compact previews use named display refs",
);
eq(
  restoreAttachmentRefsForSubmit("review @[DS30000.sl2](.kwu/attachments/clipboard-20260610-121238.444775-000002.sl2), then @[park.png](.kwu/attachments/clipboard-20260610-121238.444775-000001.png)"),
  "review @.kwu/attachments/clipboard-20260610-121238.444775-000002.sl2, then @.kwu/attachments/clipboard-20260610-121238.444775-000001.png",
  "restores named display refs for submit",
);
eq(
  formatAttachmentRefForDisplay({ path: ".kwu/attachments/clipboard-20260610-121238.444775-000001.png", name: "park.png", source: "attachment" }),
  "@[park.png](.kwu/attachments/clipboard-20260610-121238.444775-000001.png)",
  "formats attachment display refs for edit replay",
);
eq(
  formatAttachmentRefForSubmit({ path: ".kwu/attachments/clipboard-20260610-121238.444775-000001.png" }),
  "@.kwu/attachments/clipboard-20260610-121238.444775-000001.png",
  "formats raw attachment refs for edit replay submit",
);
eq(baseName("C:\\Users\\Abyss\\Desktop\\DS30000.sl2"), "DS30000.sl2", "extracts Windows path basenames");
eq(baseName("/Users/abyss/Desktop/park.png"), "park.png", "extracts POSIX path basenames");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);

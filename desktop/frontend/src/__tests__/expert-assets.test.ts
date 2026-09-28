// Run: tsx src/__tests__/expert-assets.test.ts

import { expertMemberAvatarSrc } from "../lib/expertAssets";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

console.log("\nexpert asset resolution");

const accountingAvatar = expertMemberAvatarSrc({ name: "会计专家", avatarKey: "doc-report" });
ok(
  typeof accountingAvatar === "string" && accountingAvatar.includes("doc-report"),
  "explicit member avatar key wins over finance member name defaults",
);

const uploadedAvatar = "data:image/png;base64,member";
ok(
  expertMemberAvatarSrc({ name: "审计专家", avatarKey: uploadedAvatar }) === uploadedAvatar,
  "uploaded member avatar data URL is preserved",
);

const auditAvatar = expertMemberAvatarSrc({ name: "审计专家" });
ok(
  typeof auditAvatar === "string" && auditAvatar.includes("research"),
  "member name still resolves a static fallback when no avatar key is present",
);

const remoteAvatar = "https://assets.example.com/experts/audit.png";
ok(
  expertMemberAvatarSrc({ name: "Audit Expert", avatarKey: "research", avatarUrl: remoteAvatar }) === remoteAvatar,
  "remote HTTPS avatar wins over packaged avatar fallbacks",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);

import { readFileSync } from "node:fs";
import ts from "typescript";

const owners = [
  "CloudProjectsSectionImpl.tsx",
  "ProjectAssetsPanel.tsx",
  "WorkItemBoard.tsx",
  "ProjectCapabilityDialog.tsx",
  "ProjectMemberDrawer.tsx",
  "ProjectActivityPanel.tsx",
  "ProjectAssetSelection.tsx",
];

const visibleSetter = /\bset(?:Error|Status|Message|InstructionError|DialogError|JoinError|LiveMessage)\s*\(/;
const gatewayOwner = /\bgateway\.|\bload\s*\(/;

for (const owner of owners) {
  const url = new URL(`../features/projects/ui/${owner}`, import.meta.url);
  const source = readFileSync(url, "utf8");
  const file = ts.createSourceFile(owner, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const violations: string[] = [];

  const inspectHandler = (tryText: string, handlerText: string, start: number) => {
    if (!gatewayOwner.test(tryText) || !visibleSetter.test(handlerText) || handlerText.includes('showError("projects.operation"')) return;
    const line = file.getLineAndCharacterOfPosition(start).line + 1;
    violations.push(`${owner}:${line}`);
  };

  const visit = (node: ts.Node) => {
    if (ts.isCatchClause(node)) {
      const tryStatement = node.parent;
      inspectHandler(tryStatement.tryBlock.getText(file), node.block.getText(file), node.getStart(file));
    }
    if (ts.isCallExpression(node)
      && ts.isPropertyAccessExpression(node.expression)
      && node.expression.name.text === "catch"
      && node.arguments[0]
      && (ts.isArrowFunction(node.arguments[0]) || ts.isFunctionExpression(node.arguments[0]))) {
      inspectHandler(node.expression.expression.getText(file), node.arguments[0].body.getText(file), node.getStart(file));
    }
    ts.forEachChild(node, visit);
  };
  visit(file);

  if (!source.includes('showError("projects.operation"')) throw new Error(`${owner} has no project diagnostic presentation owner`);
  if (violations.length) throw new Error(`project gateway failures discard diagnostic IDs: ${violations.join(", ")}`);
}

console.log("all project gateway UI owners preserve diagnostic presentation");

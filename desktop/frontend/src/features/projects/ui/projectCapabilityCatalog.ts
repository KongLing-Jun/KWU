import type { ProjectGateway } from "../data/projectGateway";
import type { CloudProjectCapabilityBinding } from "../model/projectTypes";
import { clientErrorCode } from "../../../lib/clientError";

export interface ProjectCapabilityCatalogItem {
  id: string;
  name: string;
  description?: string;
  meta?: string;
  category?: string;
  iconUrl?: string;
  mode?: "expert" | "team";
  memberCount?: number;
  available?: boolean;
  availabilityReason?: string;
  connectable?: boolean;
  installable?: boolean;
  localStatus?: string;
  sources?: string[];
}

export type ProjectCatalogKind = "connector" | "expert" | "skill";

export interface ProjectSkillInstallSummary {
  installed: string[];
  failed: string[];
}

function publicSkillIdentity(registryRef: string): { registryRef: string; namespace: string; slug: string } | null {
  const normalized = registryRef.trim();
  const match = /^@([^/]+)\/([^/]+)$/.exec(normalized);
  if (!match) return null;
  return { registryRef: normalized, namespace: match[1], slug: match[2] };
}

export async function installMissingBoundProjectSkills(
  bindings: readonly CloudProjectCapabilityBinding[],
  gateway: ProjectGateway,
): Promise<ProjectSkillInstallSummary> {
  const identities = Array.from(new Map(bindings
    .filter((binding) => binding.kind === "skill")
    .map((binding) => publicSkillIdentity(binding.registryRef))
    .filter((identity): identity is NonNullable<typeof identity> => identity !== null)
    .map((identity) => [identity.registryRef.toLocaleLowerCase("en-US"), identity])).values());
  if (identities.length === 0) return { installed: [], failed: [] };

  let installedAliases = new Set<string>();
  try {
    const settings = await gateway.listProjectSkillsSettings();
    installedAliases = new Set(settings.skills.flatMap((skill) => [skill.name, skill.uninstallId || ""])
      .map((value) => value.trim().toLocaleLowerCase("en-US"))
      .filter(Boolean));
  } catch {
    // Installation remains safe if the local snapshot cannot be read: the
    // installer has its own duplicate check and reports an already-installed code.
  }

  const summary: ProjectSkillInstallSummary = { installed: [], failed: [] };
  for (const identity of identities) {
    const registryKey = identity.registryRef.toLocaleLowerCase("en-US");
    const slugKey = identity.slug.toLocaleLowerCase("en-US");
    if (installedAliases.has(registryKey) || installedAliases.has(slugKey)) continue;
    try {
      await gateway.installProjectPublicSkill(identity.namespace, identity.slug);
      installedAliases.add(registryKey);
      installedAliases.add(slugKey);
      summary.installed.push(identity.registryRef);
    } catch (error) {
      if (clientErrorCode(error) === "SKILL_ALREADY_INSTALLED") {
        installedAliases.add(registryKey);
        installedAliases.add(slugKey);
        continue;
      }
      summary.failed.push(identity.registryRef);
    }
  }
  return summary;
}

export async function loadProjectCapabilityCatalog(
  kind: ProjectCatalogKind,
  gateway: ProjectGateway,
  experts: ProjectCapabilityCatalogItem[],
  boundRegistryRefs: Iterable<string> = [],
): Promise<ProjectCapabilityCatalogItem[]> {
  if (kind === "connector") {
    const connectors = await gateway.listProjectConnectorAvailability();
    const boundConnectorIDs = new Set(boundRegistryRefs);
    return connectors.map((connector) => ({
      id: connector.registryRef,
      name: connector.name,
      description: connector.description,
      iconUrl: connector.iconUrl,
      meta: connector.available ? "当前设备已连接" : connector.availabilityReason,
      available: connector.available,
      availabilityReason: connector.availabilityReason,
      connectable: connector.compatible,
      installable: connector.localStatus === "not_installed",
      localStatus: connector.localStatus,
      sources: ["all", ...(boundConnectorIDs.has(connector.registryRef) ? ["configured"] : [])],
    }));
  }

  if (kind === "expert") {
    return experts.map((expert) => ({
      ...expert,
      sources: [expert.meta === "我的专家" ? "mine" : "recommended"],
    }));
  }

  const [skillsResult, publicSkillsResult] = await Promise.allSettled([
    gateway.listProjectSkillsSettings(),
    gateway.listProjectPublicSkills(100, ""),
  ]);
  if (skillsResult.status === "rejected" && publicSkillsResult.status === "rejected") {
    throw skillsResult.reason;
  }
  const skills = skillsResult.status === "fulfilled" ? skillsResult.value : { skills: [] };
  const publicSkills = publicSkillsResult.status === "fulfilled" ? publicSkillsResult.value : { items: [] };
  const installedNames = new Set(skills.skills
    .filter((skill) => skill.enabled)
    .flatMap((skill) => [skill.name, skill.uninstallId || ""]));
  const catalogAliases = new Set<string>();
  const catalog = publicSkills.items.map((skill) => {
    const namespace = skill.namespace.trim().replace(/^@/, "").replace(/\/.+$/, "");
    const registryRef = `@${namespace}/${skill.slug}`;
    catalogAliases.add(skill.slug);
    catalogAliases.add(registryRef);
    const installed = installedNames.has(skill.slug) || installedNames.has(registryRef);
    return {
      id: registryRef,
      name: skill.name || skill.slug,
      description: skill.description,
      category: skill.categories?.[0] || skill.category || "其他",
      iconUrl: skill.iconUrl || undefined,
      meta: installed ? "当前设备已安装" : "当前设备未安装",
      available: installed,
      availabilityReason: installed ? "" : "保存到云端后将自动安装到本机",
      installable: !installed,
      sources: ["recommended", "skillhub", ...(installed ? ["installed"] : [])],
    };
  });
  const catalogIDs = new Set(catalog.map((item) => item.id));
  for (const skill of skills.skills) {
    const registryRef = (skill.uninstallId || "").trim();
    if (!skill.enabled || !registryRef || catalogIDs.has(registryRef) || catalogAliases.has(skill.name) || catalogAliases.has(registryRef)) continue;
    catalog.push({
      id: registryRef,
      name: skill.displayName || skill.name,
      description: skill.description,
      category: "本机",
      iconUrl: skill.iconUrl,
      meta: "当前设备已安装",
      available: true,
      availabilityReason: "",
      installable: false,
      sources: ["installed"],
    });
  }
  return catalog;
}

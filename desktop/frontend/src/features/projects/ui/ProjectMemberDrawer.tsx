import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react";
import { Copy, RefreshCw, UserMinus, Users, X } from "lucide-react";
import { useToast } from "../../../lib/toast";
import type { ProjectGateway } from "../data/projectGateway";
import { formatProjectDateTime } from "../model/projectDateTime";
import type { CloudProjectInvite, CloudProjectMember } from "../model/projectTypes";

interface Props { open: boolean; projectId: string; projectScopeKey: string; gateway: ProjectGateway; onClose(): void; }

export function ProjectMemberDrawer({ open, projectId, projectScopeKey, gateway, onClose }: Props) {
  const { showError } = useToast();
  const drawerRef = useRef<HTMLElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const returnFocusRef = useRef<HTMLElement | null>(null);
  const [members, setMembers] = useState<CloudProjectMember[]>([]);
  const [invites, setInvites] = useState<CloudProjectInvite[]>([]);
  const [role, setRole] = useState<"owner" | "member" | "">("");
  const [status, setStatus] = useState<{ kind: "success" | "error"; text: string } | null>(null);
  const [pending, setPending] = useState(false);

  const load = useCallback(async () => {
    const list = await gateway.listCloudProjectMembers(projectId);
    if (list.projectScopeKey !== projectScopeKey || list.projectId !== projectId) return;
    setMembers(list.items); setRole(list.currentRole);
    if (list.currentRole === "owner") {
      const pendingInvites = await gateway.listCloudProjectInvites(projectId);
      if (pendingInvites.projectScopeKey === projectScopeKey && pendingInvites.projectId === projectId) setInvites(pendingInvites.items);
    } else setInvites([]);
  }, [gateway, projectId, projectScopeKey]);

  const refresh = useCallback(async () => {
    setPending(true); setStatus(null);
    try { await load(); }
    catch (reason) {
      setStatus({ kind: "error", text: "成员与邀请加载失败，请稍后重试。" });
      showError("projects.operation", reason, "成员与邀请加载失败，请稍后重试。");
    }
    finally { setPending(false); }
  }, [load, showError]);

  useEffect(() => {
    if (!open) return;
    returnFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    queueMicrotask(() => closeRef.current?.focus());
    void refresh();
    return () => { queueMicrotask(() => returnFocusRef.current?.focus()); };
  }, [open, projectId, projectScopeKey, refresh]);

  if (!open) return null;

  const trapFocus = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "Escape") { event.preventDefault(); onClose(); return; }
    if (event.key !== "Tab") return;
    const focusable = Array.from(drawerRef.current?.querySelectorAll<HTMLElement>('button:not([disabled]), [href], input:not([disabled]), [tabindex]:not([tabindex="-1"])') || []);
    if (!focusable.length) return;
    const first = focusable[0]; const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  };

  const copyProjectId = async () => {
    setPending(true); setStatus(null);
    try {
      await navigator.clipboard.writeText(projectId);
      setStatus({ kind: "success", text: "项目 ID 已复制。" });
    } catch { setStatus({ kind: "error", text: "项目 ID 复制失败，请重试。" }); }
    finally { setPending(false); }
  };

  const deactivate = async (member: CloudProjectMember) => {
    setPending(true); setStatus(null);
    try {
      await gateway.deactivateCloudProjectMember({ projectId, userId: member.userId, expectedVersion: member.version });
      await load(); setStatus({ kind: "success", text: `已停用成员 ${member.displayName}。` });
    } catch (reason) {
      setStatus({ kind: "error", text: "成员停用失败，可能是权限或版本发生变化。" });
      showError("projects.operation", reason, "成员停用失败，可能是权限或版本发生变化。");
    }
    finally { setPending(false); }
  };

  const revoke = async (invite: CloudProjectInvite) => {
    setPending(true); setStatus(null);
    try {
      await gateway.revokeCloudProjectInvite({ projectId, inviteId: invite.inviteId, expectedVersion: invite.version });
      await load(); setStatus({ kind: "success", text: "邀请已撤销。" });
    } catch (reason) {
      setStatus({ kind: "error", text: "邀请撤销失败，可能已被使用或过期。" });
      showError("projects.operation", reason, "邀请撤销失败，可能已被使用或过期。");
    }
    finally { setPending(false); }
  };

  return <div className="cloud-projects__member-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
    <aside ref={drawerRef} className="cloud-projects__member-drawer" role="dialog" aria-modal="true" aria-labelledby="cloud-project-members-title" aria-busy={pending || undefined} data-project-member-drawer data-owner-relation={role || "unknown"} onKeyDown={trapFocus}>
      <header><div><span>项目成员</span><h3 id="cloud-project-members-title">成员与邀请</h3></div><span className="cloud-projects__member-header-actions"><button type="button" aria-label="刷新成员与邀请" disabled={pending} onClick={() => void refresh()}><RefreshCw size={17} /></button><button ref={closeRef} type="button" aria-label="关闭" onClick={onClose}><X size={18} /></button></span></header>
      <div className="cloud-projects__member-scroll">
        {status ? <p className={`cloud-projects__member-feedback is-${status.kind}`} role={status.kind === "error" ? "alert" : "status"}>{status.text}</p> : null}
        <section className="cloud-projects__member-section" aria-labelledby="cloud-project-member-list-title"><div className="cloud-projects__member-section-title"><div><h4 id="cloud-project-member-list-title">当前成员</h4><p>项目当前的所有者与活跃成员。</p></div><span>{members.length}</span></div>
          {members.length ? <ul className="cloud-projects__member-list">{members.map((member) => <li key={member.userId}><span className="cloud-projects__member-avatar" aria-hidden="true"><Users size={17} /></span><div><strong>{member.displayName}</strong><span><small className={`cloud-projects__member-badge is-${member.role}`}>{member.role === "owner" ? "所有者" : "成员"}</small></span></div>{role === "owner" && member.role === "member" ? <button type="button" disabled={pending} aria-label={`停用 ${member.displayName}`} title="停用成员" onClick={() => void deactivate(member)}><UserMinus size={16} /></button> : null}</li>)}</ul> : <p className="cloud-projects__member-empty">暂无可显示的当前成员。</p>}
        </section>
        {role === "owner" ? <div className="cloud-projects__member-owner-actions"><button type="button" className="cloud-projects__member-invite-primary" disabled={pending} onClick={() => void copyProjectId()}><Copy size={16} />复制项目 ID</button><p>将项目 ID 发送给其他用户，即可邀请其加入云项目。</p></div> : <p className="cloud-projects__member-note">只有项目所有者可以管理成员。</p>}
        {role === "owner" && invites.length ? <section className="cloud-projects__member-section" aria-labelledby="cloud-project-invites-title"><div className="cloud-projects__member-section-title"><div><h4 id="cloud-project-invites-title">待处理邀请</h4></div><span>{invites.length}</span></div><ul className="cloud-projects__invite-list">{invites.map((invite) => <li key={invite.inviteId}><div><strong>成员邀请</strong><span>有效至 {formatProjectDateTime(invite.expiresAt)}</span></div><button type="button" disabled={pending} onClick={() => void revoke(invite)}>撤销</button></li>)}</ul></section> : null}
      </div>
    </aside>
  </div>;
}

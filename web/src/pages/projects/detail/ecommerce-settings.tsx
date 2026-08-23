import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useMutation } from "@tanstack/react-query";
import { App, Button, Input, Modal, Select } from "antd";
import { Archive, Check, Save, ShieldAlert } from "lucide-react";

import { updateProject } from "@/services/api/projects";

import type { ProjectDetailViewProps } from "./shared";

const ecommerceAspectOptions = [
    { label: "9:16 · 竖屏内容", value: "9:16" },
    { label: "3:4 · 商品主图", value: "3:4" },
    { label: "1:1 · 方形商品图", value: "1:1" },
    { label: "16:9 · 横屏内容", value: "16:9" },
];

export default function EcommerceSettingsView({ detail, refreshProject }: ProjectDetailViewProps) {
    const { message } = App.useApp();
    const { project } = detail;
    const [name, setName] = useState(project.name);
    const [description, setDescription] = useState(project.description || "");
    const [aspectRatio, setAspectRatio] = useState(project.aspectRatio);
    const [archiveOpen, setArchiveOpen] = useState(false);

    useEffect(() => {
        setName(project.name);
        setDescription(project.description || "");
        setAspectRatio(project.aspectRatio);
    }, [project]);

    const dirty = useMemo(
        () => name.trim() !== project.name || description !== (project.description || "") || aspectRatio !== project.aspectRatio,
        [aspectRatio, description, name, project],
    );
    const saveMutation = useMutation({
        mutationFn: () => updateProject(project.id, { name: name.trim(), description, aspectRatio }),
        onSuccess: () => { refreshProject(); message.success("电商项目设置已保存"); },
        onError: (error) => message.error(error instanceof Error ? error.message : "项目设置保存失败"),
    });
    const archiveMutation = useMutation({
        mutationFn: () => updateProject(project.id, { status: project.status === "archived" ? "active" : "archived" }),
        onSuccess: () => { setArchiveOpen(false); refreshProject(); message.success(project.status === "archived" ? "项目已恢复" : "项目已归档"); },
        onError: (error) => message.error(error instanceof Error ? error.message : "项目状态更新失败"),
    });

    return (
        <div>
            <header className="flex items-end justify-between gap-3 pb-3">
                <div><h2 className="text-lg font-semibold">项目设置</h2><p className="mt-1 text-xs text-foreground/48">电商项目基础信息与归档管理</p></div>
                <Button type={dirty ? "primary" : "default"} icon={dirty ? <Save className="size-3.5" /> : <Check className="size-3.5" />} disabled={!dirty || !name.trim()} loading={saveMutation.isPending} onClick={() => saveMutation.mutate()}>{dirty ? "保存设置" : "已保存"}</Button>
            </header>

            <section className="py-5">
                <h3 className="mb-3 text-sm font-semibold">基础设置</h3>
                <div className="grid gap-x-4 gap-y-3 md:grid-cols-2 xl:grid-cols-4">
                    <Field label="项目名称" className="xl:col-span-2"><Input value={name} onChange={(event) => setName(event.target.value)} /></Field>
                    <Field label="默认画幅"><Select className="w-full" value={aspectRatio} options={ecommerceAspectOptions} onChange={setAspectRatio} /></Field>
                    <Field label="项目简介" className="md:col-span-2 xl:col-span-4"><Input value={description} onChange={(event) => setDescription(event.target.value)} placeholder="例如：春季袜子生活方式系列套图" /></Field>
                </div>
            </section>

            <section className="py-4">
                <div className="flex flex-col gap-3 rounded-lg bg-red-500/5 px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
                    <div className="flex min-w-0 items-center gap-2.5"><span className="grid size-7 shrink-0 place-items-center rounded bg-red-500/10 text-red-500"><Archive className="size-3.5" /></span><div className="min-w-0"><h3 className="text-sm font-medium">{project.status === "archived" ? "恢复项目" : "归档项目"}</h3><p className="mt-0.5 text-[var(--fs-label)] text-foreground/48">{project.status === "archived" ? "恢复后可继续管理资产、套图与视频任务" : "保留商品资产、画布、生产记录和结果，停止新的生成任务"}</p></div></div>
                    <Button size="small" danger={project.status !== "archived"} icon={project.status === "archived" ? <Check className="size-3.5" /> : <ShieldAlert className="size-3.5" />} onClick={() => setArchiveOpen(true)}>{project.status === "archived" ? "恢复项目" : "归档项目"}</Button>
                </div>
            </section>

            <Modal className="workspace-modal workspace-modal-compact" title={project.status === "archived" ? "恢复项目" : "归档项目"} open={archiveOpen} okText={project.status === "archived" ? "确认恢复" : "确认归档"} cancelText="取消" okButtonProps={{ danger: project.status !== "archived", loading: archiveMutation.isPending }} onCancel={() => setArchiveOpen(false)} onOk={() => archiveMutation.mutate()} styles={{ body: { paddingTop: 12 } }}>
                <p className="m-0 text-sm leading-6 text-foreground/65">{project.status === "archived" ? "恢复后项目会重新进入可编辑状态。" : "归档不会删除商品资产、画布、生产记录或历史结果。"}</p>
            </Modal>
        </div>
    );
}

function Field({ label, className = "", children }: { label: string; className?: string; children: ReactNode }) {
    return <label className={`grid gap-1.5 text-xs ${className}`}><span className="font-medium text-foreground/62">{label}</span>{children}</label>;
}

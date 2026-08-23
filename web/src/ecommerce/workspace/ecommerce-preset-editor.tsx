import { useEffect, useState } from "react";
import { App, Button, Input, Modal, Tag } from "antd";
import { LockKeyhole, Plus, Save, Trash2 } from "lucide-react";

import { saveProjectEcommercePreset, type EcommercePreset, type EcommercePresetDefinition } from "@/services/api/projects";

type Props = {
    open: boolean;
    projectId: string;
    preset?: EcommercePreset;
    onClose: () => void;
    onSaved: (preset: EcommercePreset) => void;
};

export function EcommercePresetEditor({ open, projectId, preset, onClose, onSaved }: Props) {
    const { message } = App.useApp();
    const [name, setName] = useState("");
    const [description, setDescription] = useState("");
    const [definition, setDefinition] = useState<EcommercePresetDefinition | null>(null);
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        if (!open || !preset) return;
        setName(preset.system ? `${preset.name} 副本` : preset.name);
        setDescription(preset.description);
        setDefinition(structuredClone(preset.definition));
    }, [open, preset]);

    const save = async () => {
        if (!preset || !definition || !name.trim()) {
            message.warning("请填写预设名称");
            return;
        }
        if (!definition.sceneTemplate.trim() || !definition.shotRoles.length) {
            message.warning("预设必须保留场景模板和至少一个镜头角色");
            return;
        }
        setSaving(true);
        try {
            const result = await saveProjectEcommercePreset(projectId, {
                sourceId: preset.id,
                presetKey: preset.system ? undefined : preset.presetKey,
                name: name.trim(),
                description: description.trim(),
                definition,
            });
            onSaved(result.preset);
            message.success(preset.system ? "已复制为用户预设" : "已保存新的预设版本");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "预设保存失败");
        } finally {
            setSaving(false);
        }
    };

    return (
        <Modal
            open={open}
            title={preset?.system ? "复制并编辑预设" : "编辑预设新版本"}
            width={920}
            onCancel={onClose}
            footer={<div className="flex justify-end gap-2"><Button onClick={onClose}>取消</Button><Button type="primary" icon={<Save className="size-4" />} loading={saving} onClick={() => void save()}>保存版本</Button></div>}
        >
            {!preset || !definition ? null : (
                <div className="thin-scrollbar max-h-[68vh] space-y-5 overflow-y-auto pr-1">
                    <div className="grid gap-3 sm:grid-cols-2">
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">名称</span><Input value={name} maxLength={160} onChange={(event) => setName(event.target.value)} /></label>
                        <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">执行内核</span><Input value={definition.kernel} disabled /></label>
                    </div>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">说明</span><Input.TextArea value={description} autoSize={{ minRows: 2, maxRows: 4 }} maxLength={500} onChange={(event) => setDescription(event.target.value)} /></label>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">场景模板</span><Input.TextArea value={definition.sceneTemplate} autoSize={{ minRows: 2, maxRows: 5 }} onChange={(event) => setDefinition({ ...definition, sceneTemplate: event.target.value })} /></label>
                    <label className="grid gap-1.5 text-xs"><span className="font-medium text-foreground/65">商品互动关系</span><Input.TextArea value={definition.interactionTemplate} autoSize={{ minRows: 2, maxRows: 5 }} onChange={(event) => setDefinition({ ...definition, interactionTemplate: event.target.value })} /></label>

                    <section className="border-t border-border/70 pt-4">
                        <div className="mb-3 flex items-center justify-between gap-3"><div><h3 className="text-sm font-semibold">输出镜头结构</h3><p className="mt-0.5 text-xs text-foreground/48">可调整镜头、景别与动作；运行时最多扩展到 12 个槽位。</p></div><Button size="small" icon={<Plus className="size-3.5" />} disabled={definition.shotRoles.length >= 12} onClick={() => setDefinition({ ...definition, shotRoles: [...definition.shotRoles, { key: `custom_${definition.shotRoles.length + 1}`, title: "补充镜头", framing: "中景", direction: "补充尚未覆盖的商业信息", interaction: "保持商品和场景连续", durationMs: 2200, camera: defaultCustomCamera(definition.shotRoles.map((item) => item.key)) }] })}>添加镜头</Button></div>
                        <div className="space-y-3">
                            {definition.shotRoles.map((role, index) => (
                                <div key={`${role.key}-${index}`} className="grid gap-2 border-b border-border/60 pb-3 last:border-b-0 sm:grid-cols-[36px_minmax(0,0.7fr)_minmax(0,0.8fr)_minmax(0,1.4fr)_32px] sm:items-start">
                                    <span className="grid size-8 place-items-center rounded-md bg-foreground/[.055] text-xs tabular-nums text-foreground/55">{index + 1}</span>
                                    <Input value={role.title} placeholder="镜头名称" onChange={(event) => setDefinition({ ...definition, shotRoles: definition.shotRoles.map((item, itemIndex) => itemIndex === index ? { ...item, title: event.target.value } : item) })} />
                                    <Input value={role.framing} placeholder="景别 / 机位" onChange={(event) => setDefinition({ ...definition, shotRoles: definition.shotRoles.map((item, itemIndex) => itemIndex === index ? { ...item, framing: event.target.value } : item) })} />
                                    <div className="grid gap-2"><Input.TextArea value={role.direction} autoSize={{ minRows: 1, maxRows: 3 }} placeholder="导演指令" onChange={(event) => setDefinition({ ...definition, shotRoles: definition.shotRoles.map((item, itemIndex) => itemIndex === index ? { ...item, direction: event.target.value } : item) })} /><Input.TextArea value={role.interaction} autoSize={{ minRows: 1, maxRows: 3 }} placeholder="商品互动关系" onChange={(event) => setDefinition({ ...definition, shotRoles: definition.shotRoles.map((item, itemIndex) => itemIndex === index ? { ...item, interaction: event.target.value } : item) })} /></div>
                                    <Button type="text" danger icon={<Trash2 className="size-3.5" />} aria-label="删除镜头" disabled={definition.shotRoles.length <= 1} onClick={() => setDefinition({ ...definition, shotRoles: definition.shotRoles.filter((_, itemIndex) => itemIndex !== index) })} />
                                    <details className="sm:col-start-2 sm:col-span-3">
                                        <summary className="cursor-pointer select-none py-1 text-xs font-medium text-foreground/55">专业机位</summary>
                                        <div className="mt-2 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
                                            <CameraField label="方位" value={role.camera.azimuth} onChange={(value) => setDefinition(updateRoleCamera(definition, index, "azimuth", value))} />
                                            <CameraField label="俯仰" value={role.camera.elevation} onChange={(value) => setDefinition(updateRoleCamera(definition, index, "elevation", value))} />
                                            <CameraField label="机位高度" value={role.camera.cameraHeight} onChange={(value) => setDefinition(updateRoleCamera(definition, index, "cameraHeight", value))} />
                                            <CameraField label="焦段" value={role.camera.lens} onChange={(value) => setDefinition(updateRoleCamera(definition, index, "lens", value))} />
                                            <CameraField label="距离" value={role.camera.distance} onChange={(value) => setDefinition(updateRoleCamera(definition, index, "distance", value))} />
                                            <CameraField label="主体区域" value={role.camera.subjectRegion} onChange={(value) => setDefinition(updateRoleCamera(definition, index, "subjectRegion", value))} />
                                            <CameraField label="画面占比" value={role.camera.subjectFill} onChange={(value) => setDefinition(updateRoleCamera(definition, index, "subjectFill", value))} />
                                            <CameraField label="姿态" value={role.camera.pose} onChange={(value) => setDefinition(updateRoleCamera(definition, index, "pose", value))} />
                                            <div className="sm:col-span-2 lg:col-span-4"><CameraField label="构图" value={role.camera.composition} onChange={(value) => setDefinition(updateRoleCamera(definition, index, "composition", value))} /></div>
                                        </div>
                                    </details>
                                </div>
                            ))}
                        </div>
                    </section>

                    <label className="grid gap-1.5 border-t border-border/70 pt-4 text-xs"><span className="font-medium text-foreground/65">补充排除项</span><Input.TextArea value={definition.negativePrompt} autoSize={{ minRows: 2, maxRows: 5 }} onChange={(event) => setDefinition({ ...definition, negativePrompt: event.target.value })} /></label>

                    <section className="border-t border-border/70 pt-4">
                        <div className="flex items-center gap-2 text-sm font-semibold"><LockKeyhole className="size-4 text-emerald-600" />系统锁定约束</div>
                        <p className="mt-1 text-xs leading-5 text-foreground/48">以下约束不能被用户预设删除；保存时后端会写入当前系统版本。</p>
                        <div className="mt-3 flex flex-wrap gap-1.5">{Object.values(definition.requiredConstraints).flat().map((item) => <Tag key={item} className="m-0 max-w-full !whitespace-normal !rounded-md !py-1">{item}</Tag>)}</div>
                    </section>
                </div>
            )}
        </Modal>
    );
}

type CameraFieldProps = { label: string; value: string; onChange: (value: string) => void };
type EditableCameraKey = Exclude<keyof EcommercePresetDefinition["shotRoles"][number]["camera"], "avoidReuseOf">;

function CameraField({ label, value, onChange }: CameraFieldProps) {
    return <label className="grid gap-1 text-xs"><span className="text-foreground/52">{label}</span><Input value={value} onChange={(event) => onChange(event.target.value)} /></label>;
}

function updateRoleCamera(definition: EcommercePresetDefinition, index: number, key: EditableCameraKey, value: string) {
    return {
        ...definition,
        shotRoles: definition.shotRoles.map((role, roleIndex) => roleIndex === index ? { ...role, camera: { ...role.camera, [key]: value } } : role),
    };
}

function defaultCustomCamera(avoidReuseOf: string[]): EcommercePresetDefinition["shotRoles"][number]["camera"] {
    return {
        azimuth: "opposite-side three-quarter view",
        elevation: "level",
        cameraHeight: "centered on the product",
        lens: "70mm",
        distance: "medium-close",
        subjectRegion: "product",
        subjectFill: "60-75% of frame height",
        pose: "a new product-readable moment",
        composition: "opposite-third composition with new crop boundaries",
        avoidReuseOf,
    };
}

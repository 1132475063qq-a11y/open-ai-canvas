import { useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Form, Input, Modal, Select } from "antd";
import { ArrowRight, Clock3, Images, LayoutGrid, Plus, ShoppingBag } from "lucide-react";
import { Link, useNavigate } from "react-router";

import { CollectionGrid, PageHeader, WorkspacePage } from "@/components/layout/workspace-page";
import { WorkspaceErrorState, WorkspaceLoadingState, WorkspaceState } from "@/components/layout/workspace-state";
import { createProject, listProjects, type ProjectSummary } from "@/services/api/projects";

type EcommerceProjectForm = {
    name: string;
    aspectRatio: string;
};

export default function EcommerceProjectsPage() {
    const { message } = App.useApp();
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const [createForm] = Form.useForm<EcommerceProjectForm>();
    const [createOpen, setCreateOpen] = useState(false);
    const query = useQuery({ queryKey: ["projects"], queryFn: listProjects });
    const mutation = useMutation({
        mutationFn: (values: EcommerceProjectForm) => createProject({
            name: values.name.trim(),
            type: "ecommerce",
            aspectRatio: values.aspectRatio,
            sourceType: "ecommerce",
            description: "AI 电商创意生产项目",
        }),
        onSuccess: ({ project }) => {
            setCreateOpen(false);
            createForm.resetFields();
            void queryClient.invalidateQueries({ queryKey: ["projects"] });
            navigate(`/projects/${project.id}/ecommerce`);
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "电商项目创建失败"),
    });
    const projects = (query.data?.projects || []).filter(({ project }) => project.type === "ecommerce");

    return (
        <WorkspacePage grid className="canvas-library-page">
            <div className="studio-band">
                <PageHeader
                    title="电商创意"
                    description="从商品资产到系列套图，再到图片转视频的独立生产空间。"
                    meta={<span className="app-projects-header-meta">{projects.length} 个</span>}
                    actions={<Button type="primary" icon={<Plus className="size-3.5" />} onClick={() => setCreateOpen(true)}>新建电商项目</Button>}
                />
            </div>

            {query.isError ? <WorkspaceErrorState description={query.error instanceof Error ? query.error.message : "电商项目加载失败"} onRetry={() => void query.refetch()} /> : null}
            {query.isLoading ? <WorkspaceLoadingState label="正在整理电商项目" detail="读取商品资产、生产记录与画布进度" /> : null}
            {!query.isLoading && !query.isError && projects.length ? <CollectionGrid className="project-library-grid">{projects.map((row) => <EcommerceProjectCard key={row.project.id} row={row} />)}</CollectionGrid> : null}
            {!query.isLoading && !query.isError && !projects.length ? <WorkspaceState icon="assets" title="创建第一个电商项目" description="先建立一个电商项目，再上传商品、模特、场景和品牌资产。" action={<Button type="primary" icon={<Plus className="size-3.5" />} onClick={() => setCreateOpen(true)}>创建电商项目</Button>} /> : null}

            <Modal className="library-modal" title="新建电商项目" open={createOpen} footer={null} destroyOnHidden onCancel={() => setCreateOpen(false)} width={520}>
                <Form<EcommerceProjectForm> form={createForm} layout="vertical" initialValues={{ aspectRatio: "9:16" }} onFinish={(values) => mutation.mutate(values)}>
                    <Form.Item name="name" label="项目名称" rules={[{ required: true, whitespace: true, message: "请输入项目名称" }]}><Input autoFocus placeholder="例如：春季袜子生活方式套图" /></Form.Item>
                    <Form.Item name="aspectRatio" label="默认画幅"><Select options={[{ label: "9:16 竖屏", value: "9:16" }, { label: "3:4 商品图", value: "3:4" }, { label: "1:1 方形", value: "1:1" }]} /></Form.Item>
                    <p className="-mt-1 mb-5 text-xs leading-5 text-foreground/48">创建后可在电商工作台中选择商拍预设、渠道、分辨率和输出数量。</p>
                    <div className="flex justify-end gap-2"><Button onClick={() => setCreateOpen(false)}>取消</Button><Button type="primary" htmlType="submit" loading={mutation.isPending}>创建项目</Button></div>
                </Form>
            </Modal>
        </WorkspacePage>
    );
}

function EcommerceProjectCard({ row }: { row: ProjectSummary }) {
    return (
        <Link to={`/projects/${row.project.id}/ecommerce`} className="library-card project-library-card group">
            <span className="project-library-cover">
                <span className="project-library-cover-icon"><ShoppingBag className="size-7" /></span>
                <span className="project-library-cover-scrim" />
                <span className="project-library-cover-ratio">{row.project.aspectRatio}</span>
                <span className="project-library-cover-stage">电商生产</span>
            </span>
            <span className="project-library-body">
                <span className="project-library-heading"><strong title={row.project.name}>{row.project.name}</strong><ArrowRight className="project-library-arrow size-4" /></span>
                <span className="project-library-subtitle">商品资产 · 系列套图 · 图片转视频</span>
                <span className="project-library-progress"><span><span>{row.assetCount} 项资产</span><span>进行中</span></span><i><b style={{ width: row.assetCount ? "42%" : "12%" }} /></i></span>
                <span className="project-library-stats"><ProjectCount icon={<Clock3 className="size-3.5" />} label="生产" value={row.unitCount} /><ProjectCount icon={<LayoutGrid className="size-3.5" />} label="画布" value={row.canvasCount} /><ProjectCount icon={<Images className="size-3.5" />} label="资产" value={row.assetCount} /></span>
            </span>
        </Link>
    );
}

function ProjectCount({ icon, label, value }: { icon: ReactNode; label: string; value: number }) {
    return <span className="inline-flex items-center gap-1.5" title={`${value} ${label}`}><span className="text-foreground/32">{icon}</span><strong className="font-medium tabular-nums text-foreground/65">{value}</strong><span>{label}</span></span>;
}

import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { App, Button, Dropdown, Modal, Select } from "antd";
import { ArrowDownAZ, Clock3, Download, FileUp, ListFilter, MoreHorizontal, Plus, Search, Trash2, X } from "lucide-react";

import { CollectionGrid, PageHeader, PaginationBar, WorkspacePage } from "@/components/layout/workspace-page";
import { WorkspaceLoadingState, WorkspaceState } from "@/components/layout/workspace-state";

import { readZip } from "@/lib/zip";
import { setMediaBlob } from "@/services/file-storage";
import { setImageBlob } from "@/services/image-storage";
import { CanvasCreateCard } from "@/components/canvas/canvas-project-card";
import { CanvasFolderCard } from "@/components/canvas/canvas-folder-card";
import type { CanvasExportFile } from "@/types/canvas-export";
import { useCanvasStore } from "@/stores/canvas/use-canvas-store";
import { useCanvasUiStore } from "@/stores/canvas/use-canvas-ui-store";
import { exportCanvasProjects } from "@/lib/canvas/canvas-export";
import { saveCanvasDrawing, type CanvasDrawingRenderDraft } from "@/lib/canvas/canvas-drawing-storage";
import { freeformCanvasDocuments } from "@/lib/canvas/canvas-library-domain";
import { createCanvasProjectWithRemoteSync, saveRemoteUserDataNow } from "@/services/user-data-sync";
import { listProjects } from "@/services/api/projects";

export default function CanvasPage() {
    const { message } = App.useApp();
    const navigate = useNavigate();
    const [searchParams] = useSearchParams();
    const inputRef = useRef<HTMLInputElement>(null);
    const autoOpenRef = useRef(false);
    const [keyword, setKeyword] = useState("");
    const [sort, setSort] = useState<"updated" | "name" | "nodes">("updated");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(24);
    const hydrated = useCanvasStore((state) => state.hydrated);
    const projects = useCanvasStore((state) => state.projects);
    const importProject = useCanvasStore((state) => state.importProject);
    const selectedIds = useCanvasUiStore((state) => state.selectedProjectIds);
    const setDeleteIds = useCanvasUiStore((state) => state.setDeleteProjectIds);
    const updateProject = useCanvasStore((state) => state.updateProject);
    const [associationOpen, setAssociationOpen] = useState(false);
    const [associationProjectId, setAssociationProjectId] = useState("");
    const projectQuery = useQuery({ queryKey: ["projects"], queryFn: listProjects });

    const mode = searchParams.get("mode");
    const agentMode = mode === "new" || mode === "recent" || mode === "choose";
    const handoffMode = mode === "handoff";
    const forwardedQuery = agentMode || handoffMode ? `?${searchParams.toString()}` : "";
    const enterProject = (id: string) => {
        navigate(`/canvas/${id}${forwardedQuery}`);
    };
    const freeformProjects = useMemo(() => freeformCanvasDocuments(projects), [projects]);
    const createAndEnter = () => {
        void createCanvasProjectWithRemoteSync(`自由画布 ${freeformProjects.length + 1}`).then(({ id, syncError }) => {
            if (syncError) message.warning(syncError instanceof Error ? `画布已在本地创建，云端同步失败：${syncError.message}` : "画布已在本地创建，云端同步失败");
            enterProject(id);
        });
    };
    const filteredProjects = useMemo(() => {
        const query = keyword.trim().toLowerCase();
        const values = query ? freeformProjects.filter((project) => project.title.toLowerCase().includes(query)) : [...freeformProjects];
        values.sort((a, b) => (sort === "name" ? a.title.localeCompare(b.title, "zh-CN") : sort === "nodes" ? b.nodes.length - a.nodes.length : new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()));
        return values;
    }, [freeformProjects, keyword, sort]);
    const visibleProjects = filteredProjects.slice((page - 1) * pageSize, page * pageSize);
    const showCreateCard = !keyword.trim();
    const selectedProjects = freeformProjects.filter((project) => selectedIds.includes(project.id));
    const selectedProjectIds = selectedProjects.map((project) => project.id);
    const sortLabel = sort === "name" ? "按名称" : sort === "nodes" ? "按节点" : "最近更新";
    const sortItems = [
        { key: "updated", label: "最近更新", icon: <Clock3 className="size-3.5" /> },
        { key: "name", label: "按名称", icon: <ArrowDownAZ className="size-3.5" /> },
        { key: "nodes", label: "按节点数量", icon: <ListFilter className="size-3.5" /> },
    ];
    const associateSelected = async (nextProjectId = associationProjectId) => {
        const projectId = nextProjectId || undefined;
        selectedProjectIds.forEach((id) => updateProject(id, { projectId }));
        try {
            await saveRemoteUserDataNow();
            message.success(projectId ? "已加入项目" : "已移出项目，画布仍保留");
            setAssociationOpen(false);
        } catch (error) {
            message.error(error instanceof Error ? `画布关系保存失败：${error.message}` : "画布关系保存失败");
        }
    };
    const importCanvas = async (file?: File) => {
        if (!file) return;
        try {
            const zip = await readZip(file);
            const projectFile = zip.get("projects.json");
            if (!projectFile) throw new Error("missing projects.json");
            const data = JSON.parse(await projectFile.text()) as CanvasExportFile;
            await Promise.all(
                data.projects.flatMap((project) =>
                    project.files.map(async (item) => {
                        const blob = zip.get(item.path);
                        if (!blob) return;
                        const typedBlob = blob.type ? blob : blob.slice(0, blob.size, item.mimeType);
                        await (item.storageKey.startsWith("image:") ? setImageBlob(item.storageKey, typedBlob) : setMediaBlob(item.storageKey, typedBlob));
                    }),
                ),
            );
            await Promise.all(
                data.projects.map(async (item) => {
                    const drawingEngineById = new Map((item.drawingDocuments || []).map((document) => [document.drawingId, document.engine || "tldraw"]));
                    const importedProjectId = importProject({
                        ...item.project,
                        nodes: item.project.nodes.map((node) =>
                            node.type === "drawing" && node.metadata?.drawingId ? { ...node, metadata: { ...node.metadata, drawingEngine: drawingEngineById.get(node.metadata.drawingId) || node.metadata.drawingEngine || "tldraw" } } : node,
                        ),
                    });
                    await Promise.all(
                        (item.drawingDocuments || []).map((document) => {
                            const previewFile = document.previewPath ? zip.get(document.previewPath) : undefined;
                            const preview = previewFile && !previewFile.type ? previewFile.slice(0, previewFile.size, "image/png") : previewFile;
                            const renderFile = document.generationRender?.path ? zip.get(document.generationRender.path) : undefined;
                            const renderBlob = renderFile && !renderFile.type ? renderFile.slice(0, renderFile.size, document.generationRender?.mimeType || "image/png") : renderFile;
                            const render =
                                renderBlob && document.generationRender
                                    ? ({
                                          blob: renderBlob,
                                          pageId: document.generationRender.pageId,
                                          width: document.generationRender.width,
                                          height: document.generationRender.height,
                                          mimeType: document.generationRender.mimeType,
                                          background: document.generationRender.background,
                                      } satisfies CanvasDrawingRenderDraft)
                                    : undefined;
                            const engine = document.engine || "tldraw";
                            return saveCanvasDrawing(
                                importedProjectId,
                                document.drawingId,
                                engine,
                                document.snapshot,
                                {
                                    version: 2,
                                    engine,
                                    snapshot: document.snapshot,
                                    revision: Math.max(0, document.revision - 1),
                                    updatedAt: document.updatedAt,
                                    shapeCount: document.shapeCount,
                                    pageCount: document.pageCount,
                                },
                                preview,
                                render,
                            );
                        }),
                    );
                }),
            );
            message.success(`已导入 ${data.projects.length} 个画布`);
        } catch {
            message.error("导入失败，请选择有效的画布压缩包");
        } finally {
            if (inputRef.current) inputRef.current.value = "";
        }
    };

    useEffect(() => {
        if (!hydrated || autoOpenRef.current || (mode !== "new" && mode !== "recent" && mode !== "handoff")) return;
        autoOpenRef.current = true;
        const recentProject = [...freeformProjects].sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())[0];
        if (mode === "recent" && recentProject?.id) {
            enterProject(recentProject.id);
            return;
        }
        void createCanvasProjectWithRemoteSync(`自由画布 ${freeformProjects.length + 1}`).then(({ id, syncError }) => {
            if (syncError) message.warning(syncError instanceof Error ? `画布已在本地创建，云端同步失败：${syncError.message}` : "画布已在本地创建，云端同步失败");
            enterProject(id);
        });
    }, [freeformProjects, hydrated, message, mode]);

    if (hydrated && (mode === "new" || mode === "recent" || mode === "handoff")) return <main className="flex h-full items-center justify-center bg-background text-sm text-stone-500">正在打开画布...</main>;

    return (
        <WorkspacePage grid className="canvas-library-page">
            <div className="studio-band">
                <PageHeader
                    title="画布"
                    description="把镜头、素材和想法留在同一张画布里。"
                    meta={<span className="app-projects-header-meta">{freeformProjects.length} 个</span>}
                    actions={
                        <div className="canvas-library-header-actions">
                            <Button className="canvas-library-header-action is-primary library-primary-action" type="primary" disabled={!hydrated} icon={<Plus className="size-3.5" />} onClick={createAndEnter}>
                                新建画布
                            </Button>
                            {freeformProjects.length ? (
                                <Dropdown
                                    menu={{
                                        classNames: { root: "canvas-library-actions-menu", item: "canvas-library-actions-menu-item" },
                                        items: [{ key: "delete-all", danger: true, icon: <Trash2 className="size-3.5" />, label: "删除全部画布", onClick: () => setDeleteIds(freeformProjects.map((project) => project.id)) }],
                                    }}
                                    openClassName="is-open"
                                    placement="bottomRight"
                                    trigger={["click"]}
                                >
                                    <Button className="canvas-library-header-action is-icon" aria-label="更多画布操作" title="更多操作" icon={<MoreHorizontal className="size-4" />} />
                                </Dropdown>
                            ) : null}
                            <Button className="canvas-library-header-action" disabled={!hydrated} icon={<FileUp className="size-3.5" />} onClick={() => inputRef.current?.click()}>
                                导入
                            </Button>
                        </div>
                    }
                />

                <section className="canvas-library-discovery" aria-label="画布浏览工具">
                    <div className="canvas-library-search">
                        <Search aria-hidden="true" />
                        <input
                            value={keyword}
                            placeholder="搜索画布"
                            aria-label="搜索画布"
                            onChange={(event) => {
                                setKeyword(event.target.value);
                                setPage(1);
                            }}
                        />
                        {keyword ? (
                            <button
                                type="button"
                                aria-label="清除搜索"
                                onClick={() => {
                                    setKeyword("");
                                    setPage(1);
                                }}
                            >
                                <X />
                            </button>
                        ) : null}
                    </div>
                    <div className="canvas-library-filters">
                        <Dropdown
                            trigger={["click"]}
                            placement="bottomLeft"
                            menu={{
                                items: sortItems,
                                selectedKeys: [sort],
                                onClick: ({ key }) => {
                                    setSort(key as typeof sort);
                                    setPage(1);
                                },
                            }}
                        >
                            <button type="button" className={`canvas-library-filter${sort !== "updated" ? " is-active" : ""}`} aria-label="画布排序">
                                {sort === "updated" ? <Clock3 /> : sort === "name" ? <ArrowDownAZ /> : <ListFilter />}
                                <span>{sortLabel}</span>
                            </button>
                        </Dropdown>
                        {keyword || sort !== "updated" ? (
                            <button
                                type="button"
                                className="canvas-library-reset"
                                onClick={() => {
                                    setKeyword("");
                                    setSort("updated");
                                    setPage(1);
                                }}
                            >
                                重置
                            </button>
                        ) : null}
                    </div>
                    <span className="canvas-library-count">
                        <strong>{String(filteredProjects.length).padStart(2, "0")}</strong>
                        <span>/ {String(freeformProjects.length).padStart(2, "0")} 画布</span>
                    </span>
                </section>
            </div>

            <div className="canvas-library-frame">
                {selectedProjects.length ? (
                    <div className="app-canvas-selection-toolbar mt-2 flex min-h-10 flex-wrap items-center gap-2 rounded-md border px-3 py-1.5 text-xs">
                        <strong className="mr-auto font-medium">已选 {selectedProjects.length} 个画布</strong>
                        <Button
                            size="small"
                            disabled={!hydrated || projectQuery.isLoading}
                            onClick={() => {
                                setAssociationProjectId(selectedProjects[0]?.projectId || "");
                                setAssociationOpen(true);
                            }}
                        >
                            加入项目
                        </Button>
                        <Button size="small" disabled={!hydrated} icon={<Download className="size-3.5" />} onClick={() => void exportCanvasProjects(selectedProjects, `影策画布-${selectedProjects.length}个画布`)}>
                            导出
                        </Button>
                        <Button size="small" danger disabled={!hydrated} onClick={() => setDeleteIds(selectedProjectIds)}>
                            删除
                        </Button>
                    </div>
                ) : null}

                {!hydrated ? (
                    <WorkspaceLoadingState label="正在恢复画布" detail="读取本地缓存与账号同步状态" />
                ) : showCreateCard || visibleProjects.length ? (
                    <CollectionGrid className="canvas-library-grid">
                        {showCreateCard ? <CanvasCreateCard disabled={!hydrated} onClick={createAndEnter} /> : null}
                        {visibleProjects.map((project) => (
                            <CanvasFolderCard
                                key={project.id}
                                project={project}
                                onClick={() => enterProject(project.id)}
                            />
                        ))}
                    </CollectionGrid>
                ) : (
                    <WorkspaceState icon="canvas" title="没有匹配的画布" description="换一个画布名称或重置筛选条件。" />
                )}

                <PaginationBar
                    current={page}
                    pageSize={pageSize}
                    total={filteredProjects.length}
                    pageSizeOptions={[12, 24, 48]}
                    onChange={(nextPage, nextPageSize) => {
                        setPage(nextPageSize !== pageSize ? 1 : nextPage);
                        setPageSize(nextPageSize);
                    }}
                />
            </div>

            <input ref={inputRef} type="file" accept="application/zip,.zip" className="hidden" onChange={(event) => void importCanvas(event.target.files?.[0])} />
            <Modal
                title="加入项目"
                open={associationOpen}
                okText="保存关联"
                cancelText="取消"
                okButtonProps={{ disabled: !associationProjectId, loading: projectQuery.isFetching }}
                onCancel={() => setAssociationOpen(false)}
                onOk={() => void associateSelected()}
            >
                <p className="mb-3 text-sm text-foreground/60">选中的画布会保留原有节点和本地媒体，只增加项目关联。</p>
                <Select
                    className="w-full"
                    value={associationProjectId || undefined}
                    placeholder="选择项目"
                    options={(projectQuery.data?.projects || []).map((item) => ({ label: item.project.name, value: item.project.id }))}
                    onChange={setAssociationProjectId}
                />
            </Modal>
        </WorkspacePage>
    );
}

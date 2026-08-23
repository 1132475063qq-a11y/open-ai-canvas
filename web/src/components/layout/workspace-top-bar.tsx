import { useQuery } from "@tanstack/react-query";
import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { Link, useLocation } from "react-router";

import { SystemAnnouncementCenter } from "@/components/layout/system-announcement-center";
import { WorkspaceAccountMenu } from "@/components/layout/workspace-account-menu";
import { AnimatedThemeToggler } from "@/components/ui/animated-theme-toggler";
import { getProject } from "@/services/api/projects";
import { useThemeStore } from "@/stores/use-theme-store";
import { useUserStore } from "@/stores/use-user-store";

const PAGE_TITLES: Record<string, string> = {
    home: "首页",
    create: "创作",
    projects: "短剧创作",
    ecommerce: "电商创意",
    canvas: "画布",
    tasks: "任务",
    assets: "素材",
    skills: "技能库",
    wallet: "积分中心",
    settings: "设置",
};

export function WorkspaceTopBar({ sidebarOpen, onToggleSidebar }: { sidebarOpen: boolean; onToggleSidebar: () => void }) {
    const theme = useThemeStore((state) => state.theme);
    const setTheme = useThemeStore((state) => state.setTheme);
    const user = useUserStore((state) => state.user);
    const { pathname } = useLocation();

    const pathParts = pathname.split("/").filter(Boolean);
    const slug = pathParts[0];
    const projectId = slug === "projects" ? pathParts[1] : undefined;
    const projectQuery = useQuery({ queryKey: ["project", projectId], queryFn: () => getProject(projectId || ""), enabled: Boolean(projectId), staleTime: 30_000 });
    const pageTitle = projectQuery.data?.project.type === "ecommerce" ? "电商创意" : (slug && PAGE_TITLES[slug]) || "影策";

    return (
        <header className="app-workspace-topbar flex shrink-0 items-center justify-between gap-3 px-3 sm:px-4">
            <div className="flex min-w-0 items-center gap-1.5">
                <button type="button" className="app-workspace-topbar-icon-button" aria-label={sidebarOpen ? "收起侧栏" : "展开侧栏"} onClick={onToggleSidebar}>
                    {sidebarOpen ? <PanelLeftClose className="size-4" /> : <PanelLeftOpen className="size-4" />}
                </button>
                <nav className="flex min-w-0 items-center gap-2 text-[var(--fs-caption)] text-foreground/50" aria-label="当前位置">
                    <Link to="/" className="shrink-0 font-medium text-foreground/70 transition-colors hover:text-foreground">影策</Link>
                    <span className="shrink-0 text-foreground/30">/</span>
                    <span className="truncate font-medium text-foreground">{pageTitle}</span>
                </nav>
            </div>

            <div className="flex shrink-0 items-center gap-1">
                {user ? <SystemAnnouncementCenter userId={user.id} className="app-workspace-topbar-icon-button" /> : null}
                <AnimatedThemeToggler className="app-workspace-topbar-icon-button" theme={theme} onThemeChange={setTheme} aria-label="切换主题" />
                <WorkspaceAccountMenu />
            </div>
        </header>
    );
}

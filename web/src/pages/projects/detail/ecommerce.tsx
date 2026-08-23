import { EcommerceProductionWorkspace } from "@/ecommerce/workspace/ecommerce-production-workspace";

import type { ProjectDetailViewProps } from "./shared";

export default function EcommerceWorkspaceView(props: ProjectDetailViewProps) {
    return <EcommerceProductionWorkspace detail={props.detail} refreshProject={props.refreshProject} onCreateCanvas={props.onCreateCanvas} />;
}

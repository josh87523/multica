"use client";

import { use } from "react";
import { ProjectChatWorkbench } from "@multica/views/chat";

export default function ProjectChatWorkbenchPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <ProjectChatWorkbench projectId={id} />;
}

import { CyclicNoteMonitorPanel } from "./status-panels";
import CyclicStoryMonitorPanel from "./cyclic-story-panel";
import { ActivitySupportOverview } from "./activity-overview";
import DomainWorkspace, { type WorkspaceProps } from "@/features/workspace/shared/domain-workspace";

export default function ActivitiesWorkspace(props: WorkspaceProps) {
  return (
    <DomainWorkspace
      section="activities"
      props={props}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <ActivitySupportOverview activities={[props.views.activities?.cyclicNote, props.views.activities?.cyclicStory]} />
          <CyclicNoteMonitorPanel activity={props.views.activities?.cyclicNote} />
          <CyclicStoryMonitorPanel activity={props.views.activities?.cyclicStory} />
        </div>
      )}
    />
  );
}

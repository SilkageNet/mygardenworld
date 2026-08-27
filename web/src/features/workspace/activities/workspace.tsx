import {
  CyclicNoteMonitorPanel,
  DessertMonitorPanel,
} from "./status-panels";
import CyclicStoryMonitorPanel from "./cyclic-story-panel";
import DomainWorkspace, { type WorkspaceProps } from "@/features/workspace/shared/domain-workspace";

export default function ActivitiesWorkspace(props: WorkspaceProps) {
  return (
    <DomainWorkspace
      section="activities"
      props={props}
      statusContent={(
        <div className="space-y-3 sm:space-y-4">
          <CyclicNoteMonitorPanel activity={props.views.activities?.cyclicNote} />
          <CyclicStoryMonitorPanel activity={props.views.activities?.cyclicStory} />
          <DessertMonitorPanel activity={props.views.activities?.dessert} />
        </div>
      )}
    />
  );
}

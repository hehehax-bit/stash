import React, { useEffect, useRef } from "react";
import { useJobsSubscribe } from "src/core/StashService";
import { useToast } from "src/hooks/Toast";
import { useIntl } from "react-intl";
import { JobStatus, JobStatusUpdateType } from "src/core/generated-graphql";

// All AI background jobs are queued with descriptions starting with "AI ".
const isAIJobDescription = (description?: string) =>
  !!description?.startsWith("AI ");

export const AIBackgroundJobNotifier: React.FC = () => {
  const intl = useIntl();
  const Toast = useToast();
  const notified = useRef<Set<string>>(new Set());
  const jobsSubscribe = useJobsSubscribe();

  useEffect(() => {
    if (!jobsSubscribe.data) {
      return;
    }

    const { type, job } = jobsSubscribe.data.jobsSubscribe;
    if (!isAIJobDescription(job.description)) {
      return;
    }

    const terminal =
      type === JobStatusUpdateType.Remove ||
      job.status === JobStatus.Finished ||
      job.status === JobStatus.Failed ||
      job.status === JobStatus.Cancelled;

    if (!terminal || notified.current.has(job.id)) {
      return;
    }

    notified.current.add(job.id);

    const operationName = job.description.replace(/\.\.\.$/, "");

    if (job.status === JobStatus.Failed) {
      Toast.error(
        intl.formatMessage(
          { id: "config.tasks.ai_job_failed" },
          { operation_name: operationName }
        )
      );
    } else if (job.status === JobStatus.Cancelled) {
      Toast.toast({
        variant: "warning",
        content: intl.formatMessage(
          { id: "config.tasks.ai_job_cancelled" },
          { operation_name: operationName }
        ),
      });
    } else {
      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.ai_job_finished" },
          { operation_name: operationName }
        )
      );
    }
  }, [jobsSubscribe.data, intl, Toast]);

  return null;
};

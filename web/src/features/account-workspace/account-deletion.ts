import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import type { AccountStatus } from "@/lib/api/workspace-models";

export function accountDeleting(account: Account, status?: AccountStatus) {
  return account.deletionPending || Boolean(status?.deletionPending);
}

// Deletion is irreversible. An older list/status reply must not enable a
// pending account again. Absence requests a fresh collection read; a batch may
// have been captured before the deletion command was acknowledged.
export function reconcileAccountDeletions(accounts: Account[], statuses: Map<string, AccountStatus>) {
  return accounts.flatMap((account) => {
    const status = statuses.get(account.id.toString());
    if (account.deletionPending && !status) return [account];
    if (!accountDeleting(account, status)) return [account];
    return [{ ...account, deletionPending: true, deletionFailed: status?.deletionFailed ?? account.deletionFailed, connected: false }];
  });
}

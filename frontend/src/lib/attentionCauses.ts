import type { ApplicationMethod, RALRange } from '@/api/types'

// What "Needs attention" actually means, as a pure function (issue #206,
// stories 71-73). It used to be one word fired by two unrelated conditions
// (`ral.source === 'unresolved' || method.kind === 'unresolved'`), with no
// indication of which, what it meant, or what to do.

export type AttentionCauseID = 'ral' | 'method'

export interface AttentionCause {
  id: AttentionCauseID
  title: string
  // explanation is one sentence in the user's own terms — never the name
  // of the internal state that produced it (story 73).
  explanation: string
}

// attentionCauses is the whole "what is wrong" decision, kept pure so the
// badge's behaviour is testable without opening a popover. Only causes
// actually firing come back (story 72), so an empty result means the badge
// disappears and stays a reliable signal (story 78).
export function attentionCauses(ral: RALRange, method: ApplicationMethod): AttentionCause[] {
  const causes: AttentionCause[] = []
  // 'unresolved' means the lookup couldn't even be attempted — distinct
  // from 'n/a' (it ran and found nothing) and from 'conflict' (it found
  // two figures that disagree), both of which are answers, not problems.
  if (ral.source === 'unresolved') {
    causes.push({
      id: 'ral',
      title: 'Salary',
      explanation: "Sumisura couldn't work out how much this role pays — the lookup failed rather than coming back empty.",
    })
  }
  if (method.kind === 'unresolved') {
    causes.push({
      id: 'method',
      title: 'How to apply',
      explanation: "Sumisura couldn't work out how to apply for this role, so it can't offer you the right next step.",
    })
  }
  return causes
}

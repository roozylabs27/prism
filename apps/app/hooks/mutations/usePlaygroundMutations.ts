import { useMutation } from '@tanstack/react-query';
import {
  apiSimulateRouting,
  apiReplayRouting,
  ApiRoutingSimulationReq,
  ApiRoutingReplayReq,
} from '@/lib/api';

export function useSimulateRoutingMutation() {
  return useMutation({
    mutationFn: (req: ApiRoutingSimulationReq) => apiSimulateRouting(req),
  });
}

export function useReplayRoutingMutation() {
  return useMutation({
    mutationFn: (req: ApiRoutingReplayReq = {}) => apiReplayRouting(req),
  });
}

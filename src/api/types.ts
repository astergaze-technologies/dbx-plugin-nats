export type Headers = Record<string, string>;

export interface Payload {
  data: string;
  encoding: "utf8" | "base64";
  size: number;
  truncated?: boolean;
}

export interface Overview {
  readOnly: boolean;
  server: { name: string; id: string; version: string; cluster?: string; url: string; rttMs: number; maxPayload: number; headers: boolean; tls: boolean };
  jetstream?: { memory: number; storage: number; streams: number; consumers: number; maxMemory: number; maxStore: number };
}

export interface StreamSpec {
  name: string;
  description?: string;
  subjects: string[];
  storage: string;
  retention: string;
  discard: string;
  replicas: number;
  maxMsgs: number;
  maxBytes: number;
  maxAgeSecs: number;
  maxMsgSize?: number;
  duplicatesSecs?: number;
}

export interface StreamSummary {
  name: string;
  subjects: string[];
  retention: string;
  storage: string;
  messages: number;
  bytes: number;
  consumers: number;
  firstSeq: number;
  lastSeq: number;
  created: string;
  system: boolean;
}

export interface StreamDetail extends StreamSummary {
  spec: StreamSpec;
}

export interface StoredMessage extends Payload {
  seq: number;
  subject: string;
  time: string;
  headers?: Headers;
}

export interface Consumer {
  name: string;
  durable: boolean;
  pull: boolean;
  filterSubjects: string[];
  ackPolicy: string;
  numPending: number;
  numAckPending: number;
  numRedelivered: number;
  deliveredSeq: number;
}

export interface Bucket {
  bucket: string;
  values: number;
  history: number;
  ttl: string;
  bytes: number;
  storage: string;
  compressed: boolean;
}

export interface KeyValue extends Payload {
  key: string;
  revision: number;
  created: string;
  operation: "PUT" | "DEL" | "PURGE";
}

export interface ObjectStore {
  store: string;
  description?: string;
  size: number;
  storage: string;
  sealed: boolean;
  ttl: string;
}

export interface ObjectInfo {
  name: string;
  size: number;
  chunks: number;
  digest: string;
  modified: string;
}

export interface ServiceEndpoint {
  name: string;
  subject: string;
  queueGroup?: string;
  requests: number;
  errors: number;
  lastError?: string;
  avgMs: number;
}

export interface Service {
  name: string;
  id: string;
  version: string;
  description?: string;
  started: string;
  endpoints: ServiceEndpoint[];
}

export interface Reply extends Payload {
  subject: string;
  headers?: Headers;
  elapsedMs: number;
}

export interface LiveMessage extends Payload {
  feedId: string;
  subject: string;
  reply?: string;
  headers?: Headers;
  receivedAt: string;
  dropped?: number;
}

export interface KVChange extends KeyValue {
  feedId: string;
  bucket: string;
}

// Anything MessageCard can show.
export interface MessageLike extends Payload {
  seq?: number;
  subject?: string;
  headers?: Headers;
  time?: string;
}

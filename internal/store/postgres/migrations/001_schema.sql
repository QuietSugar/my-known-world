-- 001: MyKnownWorld 完整初始结构。
--
-- 这个文件取代了旧的 001–024 历史迁移链。2026-11-26 之前共有 24 个增量迁移，
-- 它们记录了 UUID 化、业务列切换、冗余列合并等一系列演进。演进完成后这些
-- 中间步骤已无保留价值：只有开发环境和 svc 部署环境在用，且都已是最新结构。
-- 结构直接以最终形态固化在这里，新库一次建成，不再重放历史。
--
-- 与旧迁移链的差异（演进结果，不是遗漏）：
--   * 身份只有一列：nodes.id / relations.id / field_assertions.id。
--     曾经并存的 uuid 列及其 node_uuid / from_uuid / to_uuid 副本列已删除，
--     避免出现「两列存同一个值、却可能不一致」的歧义。
--   * 保留两个独立 UUID：cmdb_local_secrets.uuid（密码条目的身份），
--     graph_change_history.entity_uuid（审计快照，历史不可改写）。
--   * nodes.legacy_id 记录该节点原本来自哪个 wiki 文件路径，用于溯源。
--
-- 身份由调用方提供，数据库不再自动生成（没有 DEFAULT、没有 uuid 触发器）。
--
-- 注意：本文件不得包含 BEGIN/COMMIT，Migrate 已在外层开启事务。
-- 本文件只需在全新数据库上执行一次，Migrate 依据 schema_migrations 跳过已应用版本。

--
-- Name: account_credential_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE account_credential_details (
    node_id text NOT NULL,
    details jsonb NOT NULL
);


--
-- Name: cmdb_local_secrets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE cmdb_local_secrets (
    ref text NOT NULL,
    opaque_id text NOT NULL,
    ciphertext bytea NOT NULL,
    nonce bytea NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    uuid uuid DEFAULT gen_random_uuid() NOT NULL
);


--
-- Name: document_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE document_details (
    node_id text NOT NULL,
    details jsonb NOT NULL
);


--
-- Name: field_assertions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE field_assertions (
    id text NOT NULL,
    node_id text NOT NULL,
    field_path text NOT NULL,
    value_hash character(64) NOT NULL,
    reference text DEFAULT ''::text NOT NULL,
    source text NOT NULL,
    evidence_kind text NOT NULL,
    certainty text NOT NULL,
    asserted_at timestamp with time zone NOT NULL,
    CONSTRAINT field_assertions_value_hash_check CHECK ((value_hash ~ '^[0-9a-f]{64}$'::text))
);


--
-- Name: field_resolutions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE field_resolutions (
    resolution_id bigint NOT NULL,
    node_id text NOT NULL,
    field_path text NOT NULL,
    selected_assertion_ids jsonb NOT NULL,
    rationale_reference text NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT field_resolutions_rationale_reference_check CHECK (((length(rationale_reference) <= 1024) AND (POSITION(('
'::text) IN (rationale_reference)) = 0) AND (POSITION(('
'::text) IN (rationale_reference)) = 0))),
    CONSTRAINT field_resolutions_selected_assertion_ids_check CHECK (((jsonb_typeof(selected_assertion_ids) = 'array'::text) AND (jsonb_array_length(selected_assertion_ids) > 0)))
);


--
-- Name: field_resolutions_resolution_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE field_resolutions_resolution_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: field_resolutions_resolution_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE field_resolutions_resolution_id_seq OWNED BY field_resolutions.resolution_id;


--
-- Name: graph_change_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE graph_change_history (
    history_id bigint NOT NULL,
    run_id bigint,
    source text NOT NULL,
    entity_type text NOT NULL,
    entity_id text NOT NULL,
    action text NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    snapshot jsonb,
    entity_uuid uuid,
    CONSTRAINT graph_change_history_action_check CHECK ((action = ANY (ARRAY['created'::text, 'updated'::text, 'stale'::text, 'deleted'::text]))),
    CONSTRAINT graph_change_history_entity_type_check CHECK ((entity_type = ANY (ARRAY['node'::text, 'relation'::text, 'assertion'::text])))
);


--
-- Name: COLUMN graph_change_history.entity_uuid; Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON COLUMN graph_change_history.entity_uuid IS '实体的永久 UUID。2026-11-26 UUID 迁移时已回填所有实体仍存在的记录；为空表示该实体已被删除，保留空值以保持历史真实，不伪造 UUID。';


--
-- Name: graph_change_history_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE graph_change_history_history_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: graph_change_history_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE graph_change_history_history_id_seq OWNED BY graph_change_history.history_id;


--
-- Name: graph_mutation_operations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE graph_mutation_operations (
    operation_id bigint NOT NULL,
    action text NOT NULL,
    before_state jsonb NOT NULL,
    after_state jsonb NOT NULL,
    after_hash character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    run_id bigint,
    CONSTRAINT graph_mutation_operations_after_hash_check CHECK ((after_hash ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT graph_mutation_operations_after_state_check CHECK ((jsonb_typeof(after_state) = 'object'::text)),
    CONSTRAINT graph_mutation_operations_before_state_check CHECK ((jsonb_typeof(before_state) = 'object'::text)),
    CONSTRAINT graph_mutation_operations_metadata_check CHECK ((jsonb_typeof(metadata) = 'object'::text))
);


--
-- Name: graph_mutation_operations_operation_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE graph_mutation_operations_operation_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: graph_mutation_operations_operation_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE graph_mutation_operations_operation_id_seq OWNED BY graph_mutation_operations.operation_id;


--
-- Name: graph_operation_undos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE graph_operation_undos (
    undo_id bigint NOT NULL,
    operation_id bigint NOT NULL,
    before_hash character(64) NOT NULL,
    recorded_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT graph_operation_undos_before_hash_check CHECK ((before_hash ~ '^[0-9a-f]{64}$'::text))
);


--
-- Name: graph_operation_undos_undo_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE graph_operation_undos_undo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: graph_operation_undos_undo_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE graph_operation_undos_undo_id_seq OWNED BY graph_operation_undos.undo_id;


--
-- Name: hardware_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE hardware_details (
    node_id text NOT NULL,
    details jsonb NOT NULL
);


--
-- Name: machine_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE machine_details (
    node_id text NOT NULL,
    details jsonb NOT NULL
);


--
-- Name: network_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE network_details (
    node_id text NOT NULL,
    details jsonb NOT NULL,
    CONSTRAINT network_details_details_check CHECK ((jsonb_typeof(details) = 'object'::text))
);


--
-- Name: nodes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE nodes (
    id text NOT NULL,
    type text NOT NULL,
    name text NOT NULL,
    lifecycle text DEFAULT ''::text NOT NULL,
    last_confirmed timestamp with time zone,
    model_version integer NOT NULL,
    source text DEFAULT 'unspecified'::text NOT NULL,
    evidence_kind text DEFAULT 'unknown'::text NOT NULL,
    certainty text DEFAULT 'unknown'::text NOT NULL,
    candidate_type text DEFAULT ''::text NOT NULL,
    review_status text DEFAULT ''::text NOT NULL,
    review_questions jsonb DEFAULT '[]'::jsonb NOT NULL,
    evidence_reference text DEFAULT ''::text NOT NULL,
    legacy_id text,
    CONSTRAINT nodes_certainty_check CHECK ((certainty = ANY (ARRAY['unknown'::text, 'provisional'::text, 'confirmed'::text, 'disputed'::text, 'stale'::text]))),
    CONSTRAINT nodes_evidence_kind_check CHECK ((evidence_kind = ANY (ARRAY['unknown'::text, 'manual'::text, 'document'::text, 'observed'::text, 'derived'::text, 'imported'::text]))),
    CONSTRAINT nodes_model_version_check CHECK ((model_version > 0)),
    CONSTRAINT nodes_type_check CHECK ((type = ANY (ARRAY['Self'::text, 'Hardware'::text, 'Machine'::text, 'SoftwareProject'::text, 'SoftwareInstance'::text, 'RunningPeriod'::text, 'AccountCredential'::text, 'Product'::text, 'Document'::text, 'Runbook'::text, 'WorkItem'::text, 'Network'::text, 'ProvisionalObject'::text])))
);


--
-- Name: product_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE product_details (
    node_id text NOT NULL,
    details jsonb NOT NULL
);


--
-- Name: project_view_templates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE project_view_templates (
    id text NOT NULL,
    project_id text DEFAULT '*'::text NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    layout_type text DEFAULT 'cards'::text NOT NULL,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: relations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE relations (
    id text NOT NULL,
    from_id text NOT NULL,
    predicate text NOT NULL,
    to_id text NOT NULL,
    source text DEFAULT 'unspecified'::text NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    evidence_kind text DEFAULT 'unknown'::text NOT NULL,
    certainty text DEFAULT 'unknown'::text NOT NULL,
    CONSTRAINT relations_certainty_check CHECK ((certainty = ANY (ARRAY['unknown'::text, 'provisional'::text, 'confirmed'::text, 'disputed'::text, 'stale'::text]))),
    CONSTRAINT relations_check CHECK ((from_id <> to_id)),
    CONSTRAINT relations_evidence_kind_check CHECK ((evidence_kind = ANY (ARRAY['unknown'::text, 'manual'::text, 'document'::text, 'observed'::text, 'derived'::text, 'imported'::text]))),
    CONSTRAINT relations_predicate_check CHECK ((predicate = ANY (ARRAY['owns'::text, 'maintains'::text, 'knowsAbout'::text, 'uses'::text, 'canOperate'::text, 'routesThrough'::text, 'providesAccessTo'::text, 'hosts'::text, 'implementedFrom'::text, 'hasRunningPeriod'::text, 'hasCredential'::text, 'documents'::text, 'appliesTo'::text, 'targets'::text, 'supersedes'::text, 'dependsOn'::text, 'hasInstance'::text, 'relatedTo'::text])))
);


--
-- Name: runbook_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE runbook_details (
    node_id text NOT NULL,
    details jsonb NOT NULL
);


--
-- Name: running_periods; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE running_periods (
    node_id text NOT NULL,
    started_at timestamp with time zone,
    ended_at timestamp with time zone,
    deployed_version text DEFAULT ''::text NOT NULL,
    verification_at timestamp with time zone,
    verification_result text DEFAULT ''::text NOT NULL,
    evidence_ref text DEFAULT ''::text NOT NULL,
    CONSTRAINT running_periods_check CHECK (((started_at IS NULL) OR (ended_at IS NULL) OR (ended_at >= started_at))),
    CONSTRAINT running_periods_check1 CHECK (((verification_at IS NULL) = (verification_result = ''::text)))
);


--
-- Name: software_instance_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE software_instance_details (
    node_id text NOT NULL,
    details jsonb NOT NULL
);


--
-- Name: software_project_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE software_project_details (
    node_id text NOT NULL,
    details jsonb NOT NULL
);


--
-- Name: sync_identity_mappings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE sync_identity_mappings (
    source text NOT NULL,
    source_id text NOT NULL,
    node_id text NOT NULL
);


--
-- Name: sync_source_policies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE sync_source_policies (
    source text NOT NULL,
    priority integer DEFAULT 0 NOT NULL,
    allowed_node_types jsonb NOT NULL,
    allowed_predicates jsonb NOT NULL,
    allowed_field_paths jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT sync_source_policies_allowed_field_paths_check CHECK ((jsonb_typeof(allowed_field_paths) = 'array'::text)),
    CONSTRAINT sync_source_policies_allowed_node_types_check CHECK ((jsonb_typeof(allowed_node_types) = 'array'::text)),
    CONSTRAINT sync_source_policies_allowed_predicates_check CHECK ((jsonb_typeof(allowed_predicates) = 'array'::text)),
    CONSTRAINT sync_source_policies_priority_check CHECK (((priority >= 0) AND (priority <= 100))),
    CONSTRAINT sync_source_policies_source_check CHECK (((length(source) >= 1) AND (length(source) <= 200)))
);


--
-- Name: sync_source_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE sync_source_runs (
    run_id bigint NOT NULL,
    source text NOT NULL,
    source_hash text NOT NULL,
    parser_version text DEFAULT ''::text NOT NULL,
    applied_at timestamp with time zone DEFAULT now() NOT NULL,
    preview jsonb NOT NULL
);


--
-- Name: sync_source_runs_run_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE sync_source_runs_run_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sync_source_runs_run_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE sync_source_runs_run_id_seq OWNED BY sync_source_runs.run_id;


--
-- Name: sync_sources; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE sync_sources (
    source text NOT NULL,
    source_hash text NOT NULL,
    parser_version text DEFAULT ''::text NOT NULL,
    applied_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: work_item_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE work_item_details (
    node_id text NOT NULL,
    details jsonb NOT NULL
);


--
-- Name: field_resolutions resolution_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY field_resolutions ALTER COLUMN resolution_id SET DEFAULT nextval('field_resolutions_resolution_id_seq'::regclass);


--
-- Name: graph_change_history history_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_change_history ALTER COLUMN history_id SET DEFAULT nextval('graph_change_history_history_id_seq'::regclass);


--
-- Name: graph_mutation_operations operation_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_mutation_operations ALTER COLUMN operation_id SET DEFAULT nextval('graph_mutation_operations_operation_id_seq'::regclass);


--
-- Name: graph_operation_undos undo_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_operation_undos ALTER COLUMN undo_id SET DEFAULT nextval('graph_operation_undos_undo_id_seq'::regclass);


--
-- Name: sync_source_runs run_id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY sync_source_runs ALTER COLUMN run_id SET DEFAULT nextval('sync_source_runs_run_id_seq'::regclass);


--
-- Name: account_credential_details account_credential_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY account_credential_details
    ADD CONSTRAINT account_credential_details_pkey PRIMARY KEY (node_id);


--
-- Name: cmdb_local_secrets cmdb_local_secrets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY cmdb_local_secrets
    ADD CONSTRAINT cmdb_local_secrets_pkey PRIMARY KEY (ref);


--
-- Name: document_details document_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY document_details
    ADD CONSTRAINT document_details_pkey PRIMARY KEY (node_id);


--
-- Name: field_assertions field_assertions_id_node_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY field_assertions
    ADD CONSTRAINT field_assertions_id_node_id_key UNIQUE (id, node_id);


--
-- Name: field_resolutions field_resolutions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY field_resolutions
    ADD CONSTRAINT field_resolutions_pkey PRIMARY KEY (resolution_id);


--
-- Name: graph_change_history graph_change_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_change_history
    ADD CONSTRAINT graph_change_history_pkey PRIMARY KEY (history_id);


--
-- Name: graph_mutation_operations graph_mutation_operations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_mutation_operations
    ADD CONSTRAINT graph_mutation_operations_pkey PRIMARY KEY (operation_id);


--
-- Name: graph_operation_undos graph_operation_undos_operation_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_operation_undos
    ADD CONSTRAINT graph_operation_undos_operation_id_key UNIQUE (operation_id);


--
-- Name: graph_operation_undos graph_operation_undos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_operation_undos
    ADD CONSTRAINT graph_operation_undos_pkey PRIMARY KEY (undo_id);


--
-- Name: hardware_details hardware_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY hardware_details
    ADD CONSTRAINT hardware_details_pkey PRIMARY KEY (node_id);


--
-- Name: machine_details machine_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY machine_details
    ADD CONSTRAINT machine_details_pkey PRIMARY KEY (node_id);


--
-- Name: network_details network_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY network_details
    ADD CONSTRAINT network_details_pkey PRIMARY KEY (node_id);


--
-- Name: product_details product_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY product_details
    ADD CONSTRAINT product_details_pkey PRIMARY KEY (node_id);


--
-- Name: project_view_templates project_view_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY project_view_templates
    ADD CONSTRAINT project_view_templates_pkey PRIMARY KEY (id);


--
-- Name: runbook_details runbook_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY runbook_details
    ADD CONSTRAINT runbook_details_pkey PRIMARY KEY (node_id);


--
-- Name: running_periods running_periods_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY running_periods
    ADD CONSTRAINT running_periods_pkey PRIMARY KEY (node_id);


--
-- Name: software_instance_details software_instance_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY software_instance_details
    ADD CONSTRAINT software_instance_details_pkey PRIMARY KEY (node_id);


--
-- Name: software_project_details software_project_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY software_project_details
    ADD CONSTRAINT software_project_details_pkey PRIMARY KEY (node_id);


--
-- Name: sync_identity_mappings sync_identity_mappings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sync_identity_mappings
    ADD CONSTRAINT sync_identity_mappings_pkey PRIMARY KEY (source, source_id);


--
-- Name: sync_source_policies sync_source_policies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sync_source_policies
    ADD CONSTRAINT sync_source_policies_pkey PRIMARY KEY (source);


--
-- Name: sync_source_runs sync_source_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sync_source_runs
    ADD CONSTRAINT sync_source_runs_pkey PRIMARY KEY (run_id);


--
-- Name: sync_sources sync_sources_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sync_sources
    ADD CONSTRAINT sync_sources_pkey PRIMARY KEY (source);


--
-- Name: work_item_details work_item_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY work_item_details
    ADD CONSTRAINT work_item_details_pkey PRIMARY KEY (node_id);


--
-- Name: assertions_legacy_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX assertions_legacy_id_key ON field_assertions USING btree (id);


--
-- Name: cmdb_local_secrets_uuid_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX cmdb_local_secrets_uuid_key ON cmdb_local_secrets USING btree (uuid);


--
-- Name: field_assertions_node_field_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX field_assertions_node_field_idx ON field_assertions USING btree (node_id, field_path, asserted_at DESC);


--
-- Name: field_resolutions_node_field_time_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX field_resolutions_node_field_time_idx ON field_resolutions USING btree (node_id, field_path, recorded_at DESC, resolution_id DESC);


--
-- Name: graph_change_history_entity_time_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX graph_change_history_entity_time_idx ON graph_change_history USING btree (entity_type, entity_id, recorded_at DESC, history_id DESC);


--
-- Name: graph_mutation_operations_run_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX graph_mutation_operations_run_idx ON graph_mutation_operations USING btree (run_id) WHERE (run_id IS NOT NULL);


--
-- Name: graph_mutation_operations_time_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX graph_mutation_operations_time_idx ON graph_mutation_operations USING btree (recorded_at DESC, operation_id DESC);


--
-- Name: idx_project_view_templates_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_view_templates_project ON project_view_templates USING btree (project_id);


--
-- Name: nodes_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX nodes_id_key ON nodes USING btree (id);


--
-- Name: nodes_legacy_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX nodes_legacy_id_key ON nodes USING btree (id);


--
-- Name: nodes_single_self; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX nodes_single_self ON nodes USING btree (type) WHERE (type = 'Self'::text);


--
-- Name: relations_from_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX relations_from_idx ON relations USING btree (from_id, predicate);


--
-- Name: relations_legacy_id_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX relations_legacy_id_key ON relations USING btree (id);


--
-- Name: relations_to_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX relations_to_idx ON relations USING btree (to_id, predicate);


--
-- Name: sync_source_runs_source_time_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sync_source_runs_source_time_idx ON sync_source_runs USING btree (source, applied_at DESC, run_id DESC);


--
-- Name: graph_change_history graph_change_history_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_change_history
    ADD CONSTRAINT graph_change_history_run_id_fkey FOREIGN KEY (run_id) REFERENCES sync_source_runs(run_id) ON DELETE SET NULL;


--
-- Name: graph_mutation_operations graph_mutation_operations_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_mutation_operations
    ADD CONSTRAINT graph_mutation_operations_run_id_fkey FOREIGN KEY (run_id) REFERENCES sync_source_runs(run_id) ON DELETE SET NULL;


--
-- Name: graph_operation_undos graph_operation_undos_operation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY graph_operation_undos
    ADD CONSTRAINT graph_operation_undos_operation_id_fkey FOREIGN KEY (operation_id) REFERENCES graph_mutation_operations(operation_id);

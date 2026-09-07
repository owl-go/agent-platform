// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, expect, it } from "vitest";
import type { CLIConnectorDefinition } from "../api/client";
import { createAppI18n } from "../i18n";
import ConnectorDetails from "./ConnectorDetails.vue";

afterEach(() => document.body.replaceChildren());

it("shows a CLI Connector summary and offers editing when allowed", async () => {
  const cli = { id: "cli-1", name: "飞书 CLI", icon: "terminal", description: "读取飞书文档和日历。", installation_type: "npm", npm_package: "@larksuite/cli", npm_version: "1.0.93", npm_integrity: "sha512-test", executable: "lark-cli", authentication_driver: "feishu", state: "available", mutable: true, capabilities: [], supported_architectures: [], recommended_skills: [], recommended_skill_ids: [], conformance_runtime_digests: [], version: 1 } as CLIConnectorDefinition;
  const wrapper = mount(ConnectorDetails, { attachTo: document.body, props: { cli, canEdit: true }, global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] } });
  await flushPromises();

  expect(document.body.textContent).toContain("读取飞书文档和日历。");
  expect(document.body.textContent).toContain("@larksuite/cli@1.0.93");
  const edit = [...document.body.querySelectorAll("button")].find((button) => button.textContent?.trim() === "编辑")!;
  edit.click();
  await flushPromises();
  expect(wrapper.emitted("edit-cli")?.[0]).toEqual([cli]);
});

// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, expect, it } from "vitest";
import type { CLIConnectorDefinition, ConnectorInstallation, ConnectorPublication } from "../api/client";
import { createAppI18n } from "../i18n";
import ConnectorDetails from "./ConnectorDetails.vue";

afterEach(() => document.body.replaceChildren());

it("shows a CLI Connector summary and offers editing when allowed", async () => {
  const cli = { id: "cli-1", name: "飞书 CLI", icon: "terminal", description: "", installation_type: "npm", npm_package: "@larksuite/cli", npm_version: "1.0.93", npm_integrity: "sha512-test", executable: "lark-cli", authentication_driver: "feishu", state: "available", mutable: true, capabilities: [], supported_architectures: [], recommended_skills: [], recommended_skill_ids: [], conformance_runtime_digests: [], version: 1 } as CLIConnectorDefinition;
  const wrapper = mount(ConnectorDetails, { attachTo: document.body, props: { cli, canEdit: true }, global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] } });
  await flushPromises();

  expect(document.body.textContent).toContain("读取、检索并操作飞书文档");
  expect(document.body.textContent).toContain("@larksuite/cli@1.0.93");
  const edit = [...document.body.querySelectorAll("button")].find((button) => button.textContent?.trim() === "编辑")!;
  edit.click();
  await flushPromises();
  expect(wrapper.emitted("edit-cli")?.[0]).toEqual([cli]);
});

it("uses installed revision guidance, emits the chosen draft, and preserves plain text", async () => {
  const installation = { id: "installation-1", source: "example", state: "active", authorized: true, name: "Example", description: "Read example data", mode: "mcp", examples_zh: ["查询示例数据 <script>alert(1)</script>"], examples_en: ["Query example data"] } as ConnectorInstallation;
  const publication = { state: "available", revision: { name: "Example v2", examples_zh: ["仅新版支持的操作"] } } as ConnectorPublication;
  const wrapper = mount(ConnectorDetails, { attachTo: document.body, props: { installation, publication }, global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] } });
  await flushPromises();
  const prompt = document.body.querySelector<HTMLButtonElement>(".connector-usage-prompt")!;
  expect(prompt.textContent).toContain(installation.examples_zh![0]);
  expect(document.body.textContent).not.toContain("仅新版支持的操作");
  expect(document.body.querySelector(".connector-usage script")).toBeNull();
  prompt.click(); await flushPromises();
  expect(wrapper.emitted("use")?.[0]).toEqual([installation.examples_zh![0]]);
  wrapper.unmount();
});

it("falls back to the supplied locale and disables unavailable publication launches", async () => {
  const publication = { state: "available", revision: { name: "Example", description: "Description", conformance_available: true, examples_zh: ["查询数据"] } } as ConnectorPublication;
  const wrapper = mount(ConnectorDetails, { attachTo: document.body, props: { publication }, global: { plugins: [createAppI18n({ getItem: () => "en" }, "en")] } });
  await flushPromises();
  expect(document.body.textContent).toContain("Try these prompts");
  expect(document.body.textContent).toContain("查询数据");
  const connect = [...document.body.querySelectorAll("button")].find(button => button.textContent?.trim() === "Connect")!;
  connect.click(); expect(wrapper.emitted("connect")).toHaveLength(1);
  await wrapper.setProps({ publication: { ...publication, state: "disabled" } });
  expect(document.body.querySelector<HTMLButtonElement>(".connector-usage-prompt")!.disabled).toBe(true);
  wrapper.unmount();
});

it("uses a centered dialog with install-independent disconnect/uninstall actions and locks operations while busy", async () => {
  const installation = { id: "installation", source: "modao", state: "active", authorized: true, name: "墨刀", description: "Design", mode: "cli" } as ConnectorInstallation;
  const wrapper = mount(ConnectorDetails, { attachTo: document.body, props: { installation, canConnect: true, canDisconnect: true, canUninstall: true }, global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] } });
  await flushPromises();
  expect(document.querySelector(".connector-details.el-dialog")).not.toBeNull();
  expect(document.querySelector(".connector-details.el-drawer")).toBeNull();
  const footer = () => [...document.querySelectorAll<HTMLButtonElement>(".connector-details .el-dialog__footer button")];
  expect(footer().map(button => button.textContent?.trim())).toEqual(["卸载", "断开"]);
  footer()[1]!.click(); expect(wrapper.emitted("disconnect")).toHaveLength(1);
  footer()[0]!.click(); expect(wrapper.emitted("uninstall")).toHaveLength(1);
  await wrapper.setProps({ installation: { ...installation, authorized: false } });
  expect(footer().map(button => button.textContent?.trim())).toEqual(["卸载", "连接"]);
  await wrapper.setProps({ busy: true });
  expect(footer().every(button => button.disabled)).toBe(true);
  expect(document.querySelector<HTMLButtonElement>(".connector-usage-prompt")!.disabled).toBe(true);
  wrapper.unmount();
});

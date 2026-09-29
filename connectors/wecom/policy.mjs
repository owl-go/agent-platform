import { readFileSync, realpathSync, statSync } from 'node:fs';
import { dirname, isAbsolute, join, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const policies = JSON.parse(readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'capabilities.json'), 'utf8'));

function messageValid(body) {
  if (typeof body.chat_id !== 'string' || !body.chat_id) return false;
  if (body.msg_type === 'markdown') return typeof body.markdown?.content === 'string' && Buffer.byteLength(body.markdown.content) <= 20480;
  if (!['image', 'file', 'voice', 'video'].includes(body.msg_type)) return false;
  if (typeof body[body.msg_type]?.media_id !== 'string' || !body[body.msg_type].media_id) return false;
  if (body.msg_type !== 'video') return true;
  return (body.video.title === undefined || typeof body.video.title === 'string' && Buffer.byteLength(body.video.title) <= 128)
    && (body.video.description === undefined || typeof body.video.description === 'string' && Buffer.byteLength(body.video.description) <= 512);
}

function validWorkspacePaths(value, workspace, depth = 0) {
  if (depth > 32) return false;
  if (Array.isArray(value)) return value.every((item) => validWorkspacePaths(item, workspace, depth + 1));
  if (value === null || typeof value !== 'object') return true;
  return Object.entries(value).every(([key, item]) => {
    if (key === 'file_path') {
      if (typeof item !== 'string' || !item.startsWith(workspace + '/') || resolve(item) !== item) return false;
      try {
        const path = realpathSync(item);
        const distance = relative(realpathSync(workspace), path);
        return distance !== '..' && !distance.startsWith('..' + sep) && !isAbsolute(distance) && statSync(path).isFile();
      } catch {
        return false;
      }
    }
    return validWorkspacePaths(item, workspace, depth + 1);
  });
}

export function validateInvocation(args, workspace = '/workspace') {
  const policy = policies.find(({ command }) => command.split(' ').every((part, index) => args[index] === part));
  if (!policy) return null;
  const prefix = policy.command;
  const prefixLength = prefix.split(' ').length;
  const noArguments = prefix === 'identity whoami' || prefix === 'message aibot sessions list';
  if (noArguments) return args.length === prefixLength ? { policy, input: {} } : null;
  if (args.length !== prefixLength + 2 || args[prefixLength] !== '--json' || Buffer.byteLength(args[prefixLength + 1] ?? '') > 65536) return null;
  let input;
  try {
    input = JSON.parse(args[prefixLength + 1]);
  } catch {
    return null;
  }
  if (input === null || typeof input !== 'object' || Array.isArray(input)) return null;
  const valid = prefix === 'contact users search' ? Array.isArray(input.keywords) && input.keywords.length >= 1 && input.keywords.length <= 10
    : prefix === 'doc search' ? Array.isArray(input.keywords)
    : prefix === 'doc contents get' ? typeof input.docid === 'string' && input.docid.length > 0
    : prefix === 'todo create' ? Array.isArray(input.items) && input.items.length >= 1 && input.items.length <= 20 && input.items.every((item) => typeof item?.title === 'string' && item.title.length > 0)
    : prefix === 'message aibot send' ? messageValid(input)
    : true;
  return valid && validWorkspacePaths(input, workspace) ? { policy, input } : null;
}

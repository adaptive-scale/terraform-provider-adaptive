#!/usr/bin/env python3
"""Fail closed before Terraform execution. Never prints config/plan values."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

FAMILIES = {'adaptive_' + x for x in ('resource','endpoint','authorization','group','script','schedule','dataprotection')}
LOCAL = 'http://localhost:8080'


def validate_configuration(directory, config, prefix):
    extra = [p for p in directory.iterdir() if (p.name.endswith('.tf') or p.name.endswith('.tf.json')) and p.name != 'main.tf.json']
    if extra:
        raise ValueError('unexpected Terraform configuration files')
    if set(config) - {'terraform','provider','resource'}:
        raise ValueError('unsupported top-level fixture configuration')
    provider = config.get('provider', {})
    if set(provider) != {'adaptive'} or provider['adaptive'] != {'workspace_url': LOCAL}:
        raise ValueError('unsafe provider configuration')
    terraform = config.get('terraform', {})
    if set(terraform) != {'required_providers'} or terraform['required_providers'] != {'adaptive': {'source': 'adaptive-scale/adaptive'}}:
        raise ValueError('unexpected provider/backend configuration')
    resources = config.get('resource', {})
    if len(resources) != 1 or not set(resources) <= FAMILIES:
        raise ValueError('unsupported fixture resource family')
    objects = next(iter(resources.values()))
    if set(objects) != {'test'}:
        raise ValueError('one owned object per fixture required')
    attrs = objects['test']
    if not (attrs.get('name') or attrs.get('resource') or '').startswith(prefix + '-'):
        raise ValueError('non-owned fixture name or binding')


def validate_plan(plan, prefix, destroy):
    providers = plan.get('configuration', {}).get('provider_config', {})
    if set(providers) != {'adaptive'}:
        raise ValueError('unexpected provider configuration in saved plan')
    provider = providers['adaptive']
    if provider.get('full_name') != 'registry.terraform.io/adaptive-scale/adaptive' or provider.get('expressions') != {'workspace_url': {'constant_value': LOCAL}}:
        raise ValueError('saved plan provider is not literal local-only')
    for change in plan.get('resource_changes', []):
        if change.get('type') not in FAMILIES:
            raise ValueError('unexpected planned resource family')
        actions = change['change']['actions']
        if destroy:
            if actions not in [['delete'], ['no-op']]:
                raise ValueError('destroy plan contains non-delete actions')
        elif 'delete' in actions:
            raise ValueError('deletion/replacement blocked by manual validation hold')
        for side in ('before', 'after'):
            value = change['change'].get(side)
            if value is not None and not (value.get('name') or value.get('resource') or '').startswith(prefix + '-'):
                raise ValueError('saved plan contains a non-owned object')


def saved_plan_argument(arguments):
    """Return a saved plan, or None for an unambiguous refresh-only apply."""
    seen, positional = set(), []
    for argument in arguments[1:]:
        if not argument.startswith('-'):
            positional.append(argument)
            continue
        name, separator, value = argument.partition('=')
        if name in seen:
            raise ValueError('duplicate apply flag')
        seen.add(name)
        if name in {'-refresh-only', '-auto-approve'} and value in {'', 'true'}:
            continue
        if name == '-input' and value == 'false':
            continue
        if name == '-no-color' and not separator:
            continue
        if name == '-lock-timeout' and re.fullmatch(r'[0-9]+(?:s|m|h)', value):
            continue
        if name == '-parallelism' and value.isdigit() and int(value) > 0:
            continue
        raise ValueError('unsupported or unsafe apply flag')
    if '-refresh-only' in seen:
        if positional:
            raise ValueError('refresh-only cannot apply a saved plan')
        return None
    if len(positional) != 1:
        raise ValueError('apply requires exactly one saved plan')
    return positional[0]


def tf_environment(run):
    env = {k:v for k,v in os.environ.items() if not k.startswith('TF_CLI_ARGS') and k not in {'ADAPTIVE_SVC_TOKEN','ADAPTIVE_URL','ADAPTIVE_PROFILE','TF_LOG','TF_LOG_PATH','TF_REATTACH_PROVIDERS','TF_DATA_DIR','TF_WORKSPACE'}}
    env.update(TF_CLI_CONFIG_FILE=str(run/'terraform.rc'), TF_IN_AUTOMATION='1', CHECKPOINT_DISABLE='1')
    return env


def main(args):
    if len(args) < 3:
        raise ValueError('usage: guard.py config|command|destroy-plan RUN FIXTURE [arguments]')
    mode, runtext, fixture, *rest = args
    run = Path(runtext).resolve()
    if not re.fullmatch(r'[a-z0-9][a-z0-9_-]*', fixture):
        raise ValueError('invalid fixture identifier')
    manifest = json.loads((run/'manifest.json').read_text())
    directory = run/'work'/fixture
    if directory.is_symlink() or (directory/'main.tf.json').is_symlink():
        raise ValueError('symlinked fixture paths are unsupported')
    config = json.loads((directory/'main.tf.json').read_text())
    validate_configuration(directory, config, manifest['prefix'])
    expected = 'provider_installation {\n dev_overrides {\n "adaptive-scale/adaptive" = "%s/bin"\n }\n direct {}\n}\n' % run
    if (run/'terraform.rc').read_text() != expected:
        raise ValueError('provider CLI configuration differs from pinned local override')
    plan = None
    if mode == 'command':
        if not rest or rest[0] not in {'validate','plan','apply','import','show'}:
            raise ValueError('unsupported Terraform subcommand')
        if any(x.split('=')[0] in {'-destroy','-replace'} for x in rest):
            raise ValueError('destruction/replacement flags blocked by manual validation hold')
        if rest[0] == 'apply':
            candidate = saved_plan_argument(rest)
            if candidate is not None:
                plan = Path(candidate)
                plan = plan.resolve() if plan.is_absolute() else (directory/plan).resolve()
                if plan.parent != directory.resolve():
                    raise ValueError('normal apply plan must be inside its fixture directory')
    elif mode == 'destroy-plan':
        if len(rest) != 1:
            raise ValueError('one destroy plan required')
        plan = Path(rest[0]).resolve()
        if run/'destroy' not in plan.parents:
            raise ValueError('destroy plan must belong to this run')
    elif mode != 'config':
        raise ValueError('unsupported guard mode')
    if plan:
        result = subprocess.run(['terraform', '-chdir='+str(directory), 'show','-json',str(plan)], env=tf_environment(run), capture_output=True)
        if result.returncode:
            raise ValueError('cannot inspect saved plan')
        validate_plan(json.loads(result.stdout), manifest['prefix'], mode == 'destroy-plan')
    print('fixture guard passed')


if __name__ == '__main__':
    try:
        main(sys.argv[1:])
    except (ValueError, KeyError, OSError, TypeError):
        print('fixture guard rejected unsafe or unreadable configuration/plan', file=sys.stderr)
        raise SystemExit(1)

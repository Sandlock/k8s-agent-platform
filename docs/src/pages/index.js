import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import styles from './index.module.css';

const features = [
  {
    title: 'Ephemeral by Design',
    description:
      'Every session runs in its own Kubernetes pod. When the session ends the pod is destroyed and a warm replacement is pre-started — no state bleeds between users.',
  },
  {
    title: 'BYOK Encryption',
    description:
      'Anthropic API keys are encrypted with AES-256-GCM using a master key you control. Keys are decrypted only in-memory at claim time and never written to disk or stored in Kubernetes Secrets.',
  },
  {
    title: 'Session Persistence',
    description:
      'Claude Code\'s ~/.claude/ directory is snapshotted on exit, encrypted, and stored per user per branch. The next session resumes exactly where you left off.',
  },
  {
    title: 'NetworkPolicy Isolation',
    description:
      'Sandbox pods can only receive traffic from the control plane. Egress is limited to HTTPS and DNS. Pods cannot reach each other or your internal infrastructure.',
  },
  {
    title: 'GitHub Integration',
    description:
      'Interactive repo and branch pickers right in the CLI. Branches are sorted by newest commit. A new branch name is created locally if it doesn\'t exist upstream.',
  },
  {
    title: 'No kubeconfig Required',
    description:
      'Users interact entirely through the sandlock CLI or web dashboard. No Kubernetes access needed — the control plane handles all pod orchestration.',
  },
];

function Feature({title, description}) {
  return (
    <div className={clsx('col col--4', styles.feature)}>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

function HomepageHero() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/intro">
            Get Started →
          </Link>
          <Link
            className="button button--lg"
            style={{marginLeft: '1rem', background: 'rgba(255,255,255,0.15)', color: '#fff', border: '1px solid rgba(255,255,255,0.5)'}}
            href="https://github.com/Sandlock/k8s-agent-platform">
            View on GitHub
          </Link>
        </div>
        <div className={styles.quickInstall}>
          <code>helm upgrade --install sandlock oci://ghcr.io/sandlock/charts/sandlock</code>
        </div>
      </div>
    </header>
  );
}

export default function Home() {
  return (
    <Layout
      title="Sandlock — Kubernetes sandboxes for Claude Code"
      description="Sandlock runs Claude Code agents in isolated, ephemeral Kubernetes pods with BYOK encryption, session persistence, and GitHub integration.">
      <HomepageHero />
      <main>
        <section className={styles.features}>
          <div className="container">
            <div className="row">
              {features.map((props, idx) => (
                <Feature key={idx} {...props} />
              ))}
            </div>
          </div>
        </section>

        <section className={styles.howItWorks}>
          <div className="container">
            <Heading as="h2" className="text--center">How it works</Heading>
            <div className={styles.steps}>
              <div className={styles.step}>
                <span className={styles.stepNum}>1</span>
                <div>
                  <strong>User runs <code>sandlock create</code></strong>
                  <p>The CLI sends a request to the control plane with an optional repo URL and branch.</p>
                </div>
              </div>
              <div className={styles.step}>
                <span className={styles.stepNum}>2</span>
                <div>
                  <strong>Control plane claims a warm pod</strong>
                  <p>A <code>SandboxClaim</code> resource is created. The agent-sandbox controller assigns a pre-warmed pod from the pool in under a second.</p>
                </div>
              </div>
              <div className={styles.step}>
                <span className={styles.stepNum}>3</span>
                <div>
                  <strong>Supervisor receives the claim</strong>
                  <p>The control plane POSTs the API key, repo URL, and session snapshot to the supervisor running in the pod. The key is never stored — it lives only in memory.</p>
                </div>
              </div>
              <div className={styles.step}>
                <span className={styles.stepNum}>4</span>
                <div>
                  <strong>Claude Code starts</strong>
                  <p>The supervisor clones the repo, checks out the branch, restores the session snapshot if one exists, and launches Claude Code in a PTY.</p>
                </div>
              </div>
              <div className={styles.step}>
                <span className={styles.stepNum}>5</span>
                <div>
                  <strong>Terminal is proxied</strong>
                  <p>The control plane WebSocket proxy connects the user's terminal to the pod's PTY bridge. No direct user→pod network path exists.</p>
                </div>
              </div>
              <div className={styles.step}>
                <span className={styles.stepNum}>6</span>
                <div>
                  <strong>Session saved, pod destroyed</strong>
                  <p>On exit, the supervisor snapshots <code>~/.claude/</code>, POSTs it to the control plane for encrypted storage, then the pod is deleted.</p>
                </div>
              </div>
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}

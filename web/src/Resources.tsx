// F-15 / F-16 : liste groupée par type, filtres, actions en modales ;
// la liste d'impact revient dans la même modale — montrée, jamais
// traitée à la place de l'utilisateur (RG-80).
// Premier écran migré sur Mantine (arbitrage Angelo après banc d'essai) ;
// le MantineProvider est posé à la racine dans App.tsx.

import { useCallback, useEffect, useState } from 'react'
import {
  Alert, Badge, Button, Card, Chip, Divider, Group, Modal,
  NumberInput, Select, Stack, Text, TextInput, Title,
} from '@mantine/core'
import { api, type ImpactedLesson, type ResourceDetail, type ResourceRow, type UserContext } from './api'

const KIND_LABELS: Record<ResourceRow['kind'], string> = {
  VEHICLE: 'Véhicule',
  ROOM: 'Salle',
  TRAINING_PAD: 'Plateau',
  INSTRUCTOR: 'Formateur',
}
const KIND_ORDER: ResourceRow['kind'][] = ['VEHICLE', 'INSTRUCTOR', 'ROOM', 'TRAINING_PAD']
const KIND_PLURALS: Record<ResourceRow['kind'], string> = {
  VEHICLE: 'Véhicules',
  INSTRUCTOR: 'Formateurs',
  ROOM: 'Salles',
  TRAINING_PAD: 'Plateaux',
}

function fmtShort(iso: string): string {
  return new Date(iso).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })
}
function fmtLong(iso: string): string {
  return new Date(iso).toLocaleString('fr-FR', {
    weekday: 'short', day: 'numeric', month: 'short',
    hour: '2-digit', minute: '2-digit',
  })
}
function resourceMeta(r: ResourceRow): string {
  const meta: string[] = []
  if (r.categories.length > 0) meta.push('Catégories ' + r.categories.join(', '))
  if (r.indicative_capacity) meta.push(`${r.indicative_capacity} élèves`)
  meta.push(
    r.future_lessons > 0
      ? `${r.future_lessons} séance${r.future_lessons > 1 ? 's' : ''} à venir`
      : 'aucune séance à venir',
  )
  return meta.join(' · ')
}
function ongoingLabel(r: ResourceRow): string | null {
  const ong = r.ongoing_unavailability
  if (!ong) return null
  return (
    `${ong.reason} · depuis le ${fmtShort(ong.starts_at)}` +
    (ong.ends_at ? ` · retour prévu le ${fmtShort(ong.ends_at)}` : ' · retour inconnu')
  )
}

export default function Resources({ user }: { user: UserContext }) {
  const [rows, setRows] = useState<ResourceRow[]>([])
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [kindFilter, setKindFilter] = useState('')
  const [showArchived, setShowArchived] = useState(false)
  const [creating, setCreating] = useState(false)
  const [declaring, setDeclaring] = useState<ResourceRow | null>(null)
  const [restoring, setRestoring] = useState<ResourceRow | null>(null)
  const [detail, setDetail] = useState<ResourceDetail | null>(null)
  const canManage = user.permissions.manage_resources

  const reload = useCallback(() => {
    api.resources().then(setRows).catch((e) => setError(String(e)))
  }, [])
  useEffect(reload, [reload])

  const openDetail = (id: string) =>
    api.resourceDetail(id).then(setDetail).catch((e) => setError(String(e)))

  const q = search.trim().toLowerCase()
  const groups = KIND_ORDER.map((kind) => ({
    kind,
    label: KIND_PLURALS[kind],
    items: rows.filter(
      (r) =>
        r.kind === kind &&
        (showArchived || r.status === 'ACTIVE') &&
        (!kindFilter || r.kind === kindFilter) &&
        (!q ||
          r.label.toLowerCase().includes(q) ||
          r.categories.some((c) => c.toLowerCase().includes(q))),
    ),
  })).filter((g) => g.items.length > 0)

  return (
    <Stack p="lg" gap="md" style={{ overflowY: 'auto' }}>
      {/* wrap par défaut : sur téléphone, recherche puis filtres passent
          à la ligne au lieu de déborder */}
      <Group gap="sm">
        <Title order={2} size="h3">Ressources</Title>
        <TextInput
          placeholder="Rechercher un libellé, une catégorie…"
          value={search}
          onChange={(e) => setSearch(e.currentTarget.value)}
          style={{ flex: '1 1 220px', minWidth: 180 }}
        />
        <Select
          placeholder="Tous les types"
          clearable
          data={Object.entries(KIND_LABELS).map(([value, label]) => ({ value, label: label + 's' }))}
          value={kindFilter || null}
          onChange={(v) => setKindFilter(v ?? '')}
          w={170}
        />
        <Chip checked={showArchived} onChange={setShowArchived}>Archivées</Chip>
        <div style={{ flex: 1 }} />
        {canManage && <Button onClick={() => setCreating(true)}>Créer une ressource</Button>}
      </Group>

      {error && <Alert color="red">{error}</Alert>}

      {groups.map((g) => (
        <Stack key={g.kind} gap="xs">
          <Text size="xs" fw={700} tt="uppercase" c="dimmed">{g.label}</Text>
          {g.items.map((r) => (
            <Card key={r.id} withBorder padding="md" radius="md"
              style={{ cursor: 'pointer', opacity: r.status === 'ARCHIVED' ? 0.6 : 1 }}
              onClick={() => openDetail(r.id)}>
              <Group gap="sm">
                <Badge
                  color={r.status === 'ARCHIVED' ? 'gray' : r.out_right_now ? 'red' : 'teal'}
                  variant="light" size="lg">
                  {r.status === 'ARCHIVED' ? 'Archivée' : r.out_right_now ? 'Indisponible' : 'En service'}
                </Badge>
                {/* flex-basis large : le bloc texte prend la ligne entière
                    sur petit écran, les boutons glissent en dessous */}
                <div style={{ flex: '1 1 240px', minWidth: 0 }}>
                  <Text fw={600}>{r.label}</Text>
                  <Text size="sm" c="dimmed">{resourceMeta(r)}</Text>
                  {ongoingLabel(r) && <Text size="sm" c="orange" fw={600}>{ongoingLabel(r)}</Text>}
                </div>
                {canManage && r.status === 'ACTIVE' && (
                  <Group gap="xs" onClick={(e) => e.stopPropagation()}>
                    {r.ongoing_unavailability ? (
                      <Button size="xs" onClick={() => setRestoring(r)}>Remettre en service</Button>
                    ) : (
                      <Button size="xs" variant="default" onClick={() => setDeclaring(r)}>
                        Déclarer une indisponibilité
                      </Button>
                    )}
                  </Group>
                )}
              </Group>
            </Card>
          ))}
        </Stack>
      ))}
      {rows.length === 0 && <Text c="dimmed">Aucune ressource.</Text>}

      <Modal opened={creating} onClose={() => setCreating(false)} title="Créer une ressource">
        <CreateForm onDone={() => { setCreating(false); reload() }} />
      </Modal>
      <Modal opened={!!declaring} onClose={() => setDeclaring(null)} size="lg"
        title={declaring ? `Indisponibilité — ${declaring.label}` : ''}>
        {declaring && <DeclareForm resource={declaring} onDone={reload} onClose={() => setDeclaring(null)} />}
      </Modal>
      <Modal opened={!!restoring} onClose={() => setRestoring(null)}
        title={restoring ? `Remise en service — ${restoring.label}` : ''}>
        {restoring && <RestoreForm resource={restoring} onDone={reload} onClose={() => setRestoring(null)} />}
      </Modal>
      <Modal opened={!!detail} onClose={() => setDetail(null)} size="lg" title={detail?.label}>
        {detail && (
          <DetailView detail={detail} canManage={canManage}
            onChanged={() => { reload(); openDetail(detail.id) }} />
        )}
      </Modal>
    </Stack>
  )
}

function ImpactList({ lessons }: { lessons: ImpactedLesson[] }) {
  return (
    <Stack gap="xs">
      {lessons.map((l) => (
        <Alert key={l.id} color="yellow" p="xs">
          <Text size="sm" fw={600}>{fmtLong(l.starts_at)}</Text>
          <Text size="sm" c="dimmed">{l.students} élève(s) placé(s)</Text>
        </Alert>
      ))}
    </Stack>
  )
}

function CreateForm({ onDone }: { onDone: () => void }) {
  const [kind, setKind] = useState('VEHICLE')
  const [label, setLabel] = useState('')
  const [categories, setCategories] = useState('')
  const [capacity, setCapacity] = useState<number | string>('')
  const [error, setError] = useState<string | null>(null)

  return (
    <Stack gap="sm">
      <Select label="Type" allowDeselect={false} value={kind} onChange={(v) => setKind(v ?? 'VEHICLE')}
        data={Object.entries(KIND_LABELS).map(([value, label]) => ({ value, label }))} />
      <TextInput label="Libellé" placeholder="CE-03" value={label}
        onChange={(e) => setLabel(e.currentTarget.value)} data-autofocus />
      {kind === 'VEHICLE' && (
        <>
          <TextInput label="Catégories couvertes (séparées par des virgules)" placeholder="C, CE"
            value={categories} onChange={(e) => setCategories(e.currentTarget.value)} />
          <NumberInput label="Capacité indicative en élèves" min={1} value={capacity} onChange={setCapacity} />
        </>
      )}
      {error && <Alert color="red">{error}</Alert>}
      <Group>
        <Button disabled={!label.trim()}
          onClick={() =>
            api.createResource({
              kind,
              label: label.trim(),
              ...(kind === 'VEHICLE' && categories.trim()
                ? { categories: categories.split(',').map((c) => c.trim()).filter(Boolean) }
                : {}),
              ...(kind === 'VEHICLE' && capacity ? { indicative_capacity: Number(capacity) } : {}),
            }).then(onDone).catch((e) => setError(String(e)))
          }>
          Créer
        </Button>
      </Group>
    </Stack>
  )
}

function DeclareForm({ resource, onDone, onClose }: {
  resource: ResourceRow; onDone: () => void; onClose: () => void
}) {
  const [reason, setReason] = useState('')
  const [start, setStart] = useState('')
  const [end, setEnd] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [impact, setImpact] = useState<ImpactedLesson[] | null>(null)

  if (impact !== null) {
    return (
      <Stack gap="sm">
        {impact.length === 0 ? (
          <Text c="dimmed">Indisponibilité déclarée. Aucune séance planifiée sur la période.</Text>
        ) : (
          <>
            <Text>
              Indisponibilité déclarée. <b>{impact.length} séance(s)</b> restent assignées à{' '}
              {resource.label} — à traiter dans le planning :
            </Text>
            <ImpactList lessons={impact} />
          </>
        )}
        <Group><Button onClick={onClose}>Fermer</Button></Group>
      </Stack>
    )
  }

  return (
    <Stack gap="sm">
      <TextInput label="Motif" placeholder="Panne, contrôle technique…" value={reason}
        onChange={(e) => setReason(e.currentTarget.value)} data-autofocus />
      <TextInput label="Début" type="datetime-local" value={start}
        onChange={(e) => setStart(e.currentTarget.value)} />
      <TextInput label="Fin — laisser vide si la date de retour est inconnue" type="datetime-local"
        value={end} onChange={(e) => setEnd(e.currentTarget.value)} />
      {error && <Alert color="red">{error}</Alert>}
      <Group>
        <Button disabled={!reason.trim() || !start}
          onClick={() =>
            api.declareUnavailability(resource.id, {
              reason: reason.trim(),
              starts_at: new Date(start).toISOString(),
              ...(end ? { ends_at: new Date(end).toISOString() } : {}),
            }).then((r) => { setImpact(r.impacted_lessons); onDone() })
              .catch((e) => setError(String(e)))
          }>
          Déclarer
        </Button>
        <Button variant="default" onClick={onClose}>Annuler</Button>
      </Group>
      <Text size="sm" c="dimmed">
        Les séances concernées gardent leur assignation et s'affichent en conflit — rien n'est
        annulé à votre place.
      </Text>
    </Stack>
  )
}

function RestoreForm({ resource, onDone, onClose }: {
  resource: ResourceRow; onDone: () => void; onClose: () => void
}) {
  const [note, setNote] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [kept, setKept] = useState<ImpactedLesson[] | null>(null)

  if (kept !== null) {
    return (
      <Stack gap="sm">
        <Text><b>{resource.label}</b> est de retour en service.</Text>
        {kept.length === 0 ? (
          <Text c="dimmed">Aucune séance à venir ne comptait sur cette ressource.</Text>
        ) : (
          <>
            <Text c="dimmed">
              {kept.length} séance{kept.length > 1 ? 's' : ''} étaient restées assignées pendant
              l'indisponibilité — elles redeviennent valides telles quelles :
            </Text>
            <ImpactList lessons={kept} />
          </>
        )}
        <Group><Button onClick={onClose}>Fermer</Button></Group>
      </Stack>
    )
  }

  return (
    <Stack gap="sm">
      {ongoingLabel(resource) && <Text c="dimmed">{ongoingLabel(resource)}</Text>}
      <TextInput label="Motif — facultatif" placeholder="Réparation terminée, contrôle passé…"
        value={note} onChange={(e) => setNote(e.currentTarget.value)} data-autofocus />
      {error && <Alert color="red">{error}</Alert>}
      <Group>
        <Button
          onClick={() =>
            api.restoreUnavailability(resource.ongoing_unavailability!.id, note.trim() || undefined)
              .then((r) => { setKept(r.kept_lessons); onDone() })
              .catch((e) => setError(String(e)))
          }>
          Remettre en service
        </Button>
        <Button variant="default" onClick={onClose}>Annuler</Button>
      </Group>
    </Stack>
  )
}

function DetailView({ detail, canManage, onChanged }: {
  detail: ResourceDetail; canManage: boolean; onChanged: () => void
}) {
  const [error, setError] = useState<string | null>(null)
  return (
    <Stack gap="sm">
      {error && <Alert color="red">{error}</Alert>}
      <Text size="xs" fw={700} tt="uppercase" c="dimmed">
        Séances à venir ({detail.upcoming_lessons.length})
      </Text>
      {detail.upcoming_lessons.length === 0 && <Text c="dimmed">Aucune séance planifiée.</Text>}
      {detail.upcoming_lessons.map((l) => (
        <Group key={l.id} justify="space-between">
          <Text fw={600} size="sm">{fmtLong(l.starts_at)}</Text>
          <Text size="sm" c="dimmed">{l.students} élève(s) placé(s)</Text>
        </Group>
      ))}
      <Divider />
      <Text size="xs" fw={700} tt="uppercase" c="dimmed">Indisponibilités</Text>
      {detail.unavailabilities.length === 0 && <Text c="dimmed">Aucune indisponibilité déclarée.</Text>}
      {detail.unavailabilities.map((u) => (
        <Group key={u.id} justify="space-between" wrap="nowrap">
          <div style={{ minWidth: 0 }}>
            <Text fw={600} size="sm">{u.reason}</Text>
            <Text size="sm" c="dimmed">
              {fmtLong(u.starts_at)} → {u.ends_at ? fmtLong(u.ends_at) : 'retour inconnu'}
            </Text>
            {u.restored_note && <Text size="sm" c="dimmed">Remise en service : {u.restored_note}</Text>}
          </div>
          <Badge color={u.status === 'ONGOING' ? 'red' : 'gray'} variant="light">
            {u.status === 'ONGOING' ? 'En cours' : 'Terminée'}
          </Badge>
        </Group>
      ))}
      {canManage && (
        <Group mt="sm">
          <Button variant="default"
            onClick={() =>
              api.patchResource(detail.id, {
                status: detail.status === 'ACTIVE' ? 'ARCHIVED' : 'ACTIVE',
              }).then(onChanged).catch((e) => setError(String(e)))
            }>
            {detail.status === 'ACTIVE' ? 'Archiver cette ressource' : 'Restaurer cette ressource'}
          </Button>
        </Group>
      )}
    </Stack>
  )
}

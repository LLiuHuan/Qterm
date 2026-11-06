export const remoteEditUiFr: Record<string, string> = {
  'remote.ui.openWithLocal': 'Ouvrir avec un programme local',
  'remote.ui.openInEditor': 'Éditer',
  'remote.ui.syncNow': 'Synchroniser maintenant',
  'remote.ui.stopSync': 'Arrêter la synchronisation',
  'remote.ui.discardLocal': 'Abandonner les modifications locales',
  'remote.ui.state.watching': 'En attente de modifications locales',
  'remote.ui.state.syncing': 'Synchronisation en cours',
  'remote.ui.state.synced': 'Synchronisé',
  'remote.ui.state.conflict': 'Conflit',
  'remote.ui.state.error': 'Erreur',
  'remote.ui.state.stopped': 'Arrêté',
  'remote.ui.stateLabel': 'État de synchronisation : {state}',
  'remote.ui.conflict.title': 'Le fichier distant a été modifié',
  'remote.ui.conflict.description':
    'Le contenu distant diffère de celui présent au début de votre édition. L’écrasement n’a lieu que si le contenu distant correspond toujours au contenu ci-dessous ; s’il a changé de nouveau, cette boîte de dialogue réapparaît et le contenu plus récent n’est jamais écrasé en silence.',
  'remote.ui.conflict.reload': 'Recharger',
  'remote.ui.conflict.reloadHint':
    'Abandonner les modifications locales et relire le contenu le plus récent depuis le distant',
  'remote.ui.conflict.overwrite': 'Écraser',
  'remote.ui.conflict.overwriteHint': 'Écrire le contenu local sur le fichier distant',
  'remote.ui.conflict.cancel': 'Annuler',
  'remote.ui.conflict.cancelHint': 'Conserver l’état actuel et décider plus tard',
  'remote.ui.conflict.externalNotice':
    'Cette modification a été effectuée dans un éditeur externe. Le fichier temporaire local n’étant pas lisible depuis un autre processus, seule la version distante est affichée et aucune comparaison n’est disponible.',
  'remote.ui.conflict.diffTitle': 'Contenu distant ↔ modifications locales',
  'remote.ui.conflict.diffIdentical': 'Les deux côtés sont identiques',
  'remote.ui.conflict.diffSummary': '{added} lignes ajoutées, {removed} lignes supprimées',
  'remote.ui.conflict.diffUnavailable':
    'Le contenu distant n’a pas pu être lu (binaire ou lecture incomplète) : aucune comparaison n’est affichée.',
  'remote.ui.conflict.repeated':
    'Le fichier distant a changé de nouveau pendant votre choix : rien n’a été écrit. Veuillez choisir à nouveau.',
  'remote.ui.editor.loading': 'Chargement…',
  'remote.ui.editor.save': 'Enregistrer',
  'remote.ui.editor.close': 'Fermer',
  'remote.ui.editor.refresh': 'Actualiser',
  'remote.ui.editor.refreshHint':
    'Actualiser : abandonner les modifications locales non encore synchronisées avec le distant et relire le contenu le plus récent (les modifications abandonnées ne peuvent pas être récupérées).',
  'remote.ui.editor.dirty': 'Modifications non enregistrées',
  'remote.ui.editor.clean': 'Aucune modification non enregistrée',
  'remote.ui.editor.saved': 'Enregistré sur l’hôte distant',
  'remote.ui.editor.unsupportedBinary':
    'Ce fichier est binaire et ne peut pas être édité dans l’application. Utilisez plutôt « Ouvrir avec un programme local ».',
  'remote.ui.editor.unsupportedTruncated':
    'Le fichier dépasse la limite de lecture de l’édition intégrée (8 MiB) et n’a pas été lu en entier. Utilisez plutôt « Ouvrir avec un programme local ».',
  'remote.ui.editor.unsupportedTooLarge':
    'Le fichier dépasse la limite de taille de l’édition intégrée. Utilisez plutôt « Ouvrir avec un programme local ».',
  'remote.ui.editor.fallback':
    'L’éditeur intégré n’a pas pu se charger : un éditeur de texte brut est utilisé à la place.',
  'remote.ui.editor.discardTitle': 'Abandonner les modifications locales',
  'remote.ui.editor.discardMessage':
    'Le contenu de l’éditeur sera rétabli sur le contenu distant actuel ; les modifications non enregistrées seront perdues.',
  'remote.ui.editor.discardConfirm': 'Abandonner',
  'remote.ui.editor.closeDirtyTitle': 'Abandonner les modifications avant de fermer ?',
  'remote.ui.editor.closeDirtyMessage':
    'Les modifications de cette fenêtre seront perdues à la fermeture ; le fichier distant ne sera pas modifié.',
  'remote.ui.editor.closeDirtyConfirm': 'Abandonner et fermer',
  'remote.ui.stop.title': 'Arrêter la synchronisation',
  'remote.ui.stop.message':
    'Qterm arrêtera la surveillance des modifications locales et supprimera le fichier temporaire local ; les modifications non encore synchronisées avec le distant seront perdues.',
  'remote.ui.stop.confirm': 'Arrêter et supprimer',
  'remote.ui.toast.openedLocal': 'Ouvert avec un programme local : {path}',
  'remote.ui.toast.openFailed': 'Échec de l’ouverture : {reason}',
  'remote.ui.toast.synced': 'Synchronisé avec l’hôte distant',
  'remote.ui.toast.syncFailed': 'Échec de la synchronisation : {reason}',
  'remote.ui.toast.stopped': 'Synchronisation arrêtée et fichier temporaire local supprimé',
  'remote.ui.toast.stopFailed': 'Échec de l’arrêt de la synchronisation : {reason}',
  'remote.ui.toast.saveFailed': 'Échec de l’enregistrement : {reason}',
}

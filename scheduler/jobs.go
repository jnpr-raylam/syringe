package scheduler

import (
	"fmt"
	"strconv"

	log "github.com/Sirupsen/logrus"
	pb "github.com/nre-learning/syringe/api/exp/generated"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1client "k8s.io/client-go/kubernetes/typed/batch/v1"
)

func (ls *LessonScheduler) isCompleted(job *batchv1.Job, req *LessonScheduleRequest) (bool, error) {

	nsName := fmt.Sprintf("%d-%s-ns", req.LessonDef.LessonID, req.Session)

	batchclient, err := batchv1client.NewForConfig(ls.Config)
	if err != nil {
		panic(err)
	}

	result, err := batchclient.Jobs(nsName).Get(job.Name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Couldn't retrieve job: %s", err)
		return false, err
	}
	// https://godoc.org/k8s.io/api/batch/v1#JobStatus
	log.WithFields(log.Fields{
		"jobName":    result.Name,
		"active":     result.Status.Active,
		"successful": result.Status.Succeeded,
		"failed":     result.Status.Failed,
	}).Info("Job Status")

	if result.Status.Failed > 0 {
		log.Errorf("Problem configuring with %s", result.Name)
	}

	return (result.Status.Active == 0), nil

}

func (ls *LessonScheduler) configureDevice(ep *pb.Endpoint, req *LessonScheduleRequest) (*batchv1.Job, error) {

	batchclient, err := batchv1client.NewForConfig(ls.Config)
	if err != nil {
		panic(err)
	}

	nsName := fmt.Sprintf("%d-%s-ns", req.LessonDef.LessonID, req.Session)

	jobName := fmt.Sprintf("config-%s", ep.Name)
	podName := fmt.Sprintf("config-%s", ep.Name)

	configJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: nsName,
			Labels: map[string]string{
				"lessonId":       fmt.Sprintf("%d", req.LessonDef.LessonID),
				"sessionId":      req.Session,
				"syringeManaged": "yes",
				"stageId":        strconv.Itoa(int(req.Stage)),
			},
		},

		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: nsName,
					Labels: map[string]string{
						"lessonId":       fmt.Sprintf("%d", req.LessonDef.LessonID),
						"sessionId":      req.Session,
						"syringeManaged": "yes",
						"stageId":        strconv.Itoa(int(req.Stage)),
					},
				},
				Spec: corev1.PodSpec{

					InitContainers: []corev1.Container{
						{
							Name:  "git-clone",
							Image: "alpine/git",
							Command: []string{
								"/usr/local/git/git-clone.sh",
							},
							Args: []string{
								"https://github.com/nre-learning/antidote.git",
								"master",
								"/antidote",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "git-clone",
									ReadOnly:  false,
									MountPath: "/usr/local/git",
								},
								{
									Name:      "git-volume",
									ReadOnly:  false,
									MountPath: "/antidote",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "napalm",
							Image: "antidotelabs/napalm",
							Command: []string{
								"napalm",
								"--user=root",
								"--password=VR-netlab9",
								"--vendor=junos",
								fmt.Sprintf("--optional_args=port=%d", ep.Port),
								"vip.labs.networkreliability.engineering",
								"configure",

								// TODO need to get this from syringe file
								fmt.Sprintf("/antidote/lessons/lesson-%d/stage%d/configs/%s.txt", req.LessonDef.LessonID, req.Stage, ep.Name),
								"--strategy=merge",
							},

							// TODO(mierdin): ONLY for test/dev. Should re-evaluate for prod
							ImagePullPolicy: "Always",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "git-volume",
									ReadOnly:  false,
									MountPath: "/antidote",
								},
							},
						},
					},
					RestartPolicy: "Never",
					Volumes: []corev1.Volume{
						{
							Name: "git-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "git-clone",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "git-clone",
									},
									DefaultMode: &defaultGitFileMode,
								},
							},
						},
					},
				},
			},
		},
	}

	result, err := batchclient.Jobs(nsName).Create(configJob)
	if err == nil {
		log.WithFields(log.Fields{
			"namespace": nsName,
		}).Infof("Created job: %s", result.ObjectMeta.Name)

	} else if apierrors.IsAlreadyExists(err) {
		log.Warnf("Job %s already exists.", jobName)

		result, err := batchclient.Jobs(nsName).Get(jobName, metav1.GetOptions{})
		if err != nil {
			log.Errorf("Couldn't retrieve job after failing to create a duplicate: %s", err)
			return nil, err
		}
		return result, nil
	} else {
		log.Errorf("Problem creating job %s: %s", jobName, err)
		return nil, err
	}
	return result, err
}

// ---
// apiVersion: batch/v1
// kind: Job
// metadata:
//   name: configure-lab0-vqfx1
// spec:
//   template:
//     metadata:
//       name: napalm
//     spec:
//       initContainers:
//       - name: git-clone
//         image: alpine/git # Any image with git will do
//         command:
//         - /usr/local/git/git-clone.sh
//         args:
//         - "https://github.com/nre-learning/antidote.git"
//         - "master"
//         - "/antidote"
//         volumeMounts:
//         - name: git-clone
//           mountPath: /usr/local/git
//         - name: git-volume
//           mountPath: /antidote

//       containers:
//       - name: napalm
//         image: antidotelabs/napalm
//         command:
//          - napalm
//          - --user=root
//          - --password=VR-netlab9
//          - --vendor=junos
//          - --optional_args=port=30021
//          - vip.labs.networkreliability.engineering
//          - configure
//          - /antidote/platform/sharedlab/vqfx1.txt
//          - --strategy=merge
//         volumeMounts:
//           - mountPath: /antidote
//             name: git-volume
//       volumes:
//         - name: git-volume
//           emptyDir: {}
//         - name: git-clone
//           configMap:
//             name: git-clone
//             defaultMode: 0755
//       restartPolicy: Never

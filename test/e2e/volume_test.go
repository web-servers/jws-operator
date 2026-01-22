package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("VolumeTest", Ordered, func() {
	// Constants for test
	const (
		testFileName = "test-data.txt"
	)

	name := "volume-test"
	standardClass := "standard-csi"
	appName := "jws-img"

	var webserver *webserversv1alpha1.WebServer

	BeforeEach(func() {
		webserver = &webserversv1alpha1.WebServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: webserversv1alpha1.WebServerSpec{
				ApplicationName: appName,
				Replicas:        1,
				WebImage: &webserversv1alpha1.WebImageSpec{
					ApplicationImage: testImg,
				},
				Volume: &webserversv1alpha1.VolumeSpec{},
			},
		}
	})

	AfterEach(func() {
		if webserver != nil {
			deleteWebServer(webserver)
		}
		deleteAllPVCs(namespace)

		time.Sleep(5 * time.Second)
	})

	// Waits for at least one pod to appear and returns its name.
	waitForPod := func() string {
		var podName string
		Eventually(func() bool {
			currentWebServer := getWebServer(name)
			if currentWebServer == nil || len(currentWebServer.Status.Pods) == 0 {
				return false
			}
			podName = currentWebServer.Status.Pods[0].Name
			return true
		}, time.Minute*2, time.Second).Should(BeTrue(), "Pod should appear in status")
		return podName
	}

	resolveMountPath := func(podName string) string {
		var mountPath string
		pod := &corev1.Pod{}

		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: namespace}, pod)
			if err != nil {
				return false
			}

			if len(pod.Spec.Containers) > 0 {
				for _, mount := range pod.Spec.Containers[0].VolumeMounts {
					if !strings.Contains(mount.MountPath, "/var/run/secrets") {
						mountPath = mount.MountPath
						return true
					}
				}
			}
			return false
		}, time.Second, "1s").Should(BeTrue(), "Could not find a valid volume mount in pod %s specs", podName)

		fmt.Printf("Resolved mount path for %s via API: %s\n", podName, mountPath)
		return mountPath
	}

	writeDataToFile := func(podName, containerName, fullPath, content string) {
		cmd := []string{"/bin/sh", "-c", fmt.Sprintf("echo '%s' > %s && sync", content, fullPath)}
		Eventually(func() error {
			_, stderr, err := executeCommandOnPod(podName, containerName, cmd)
			if err != nil {
				return fmt.Errorf("err: %v, stderr: %s", err, stderr)
			}
			return nil
		}, time.Minute*2, time.Second*5).Should(Succeed(), "Failed to write data to %s", fullPath)
	}

	verifyFileContent := func(podName, containerName, fullPath, expectedContent string) {
		Eventually(func() string {
			stdout, _, err := executeCommandOnPod(podName, containerName, []string{"cat", fullPath})
			if err != nil {
				return ""
			}
			return strings.TrimSpace(stdout)
		}, time.Minute*2, time.Second*5).Should(Equal(expectedContent), "File content mismatch")
	}

	Context("AccessModeTests", func() {
		/*
			Test: ReadWriteOnce (RWO)
			Goal: Verify standard volume mounting and persistence.
		*/
		It("ReadWriteOnceTest", func() {
			testContent := "RWO-Template-Check"

			By("Creating WebServer with VolumeClaimTemplate")

			pvcSpec := corev1.PersistentVolumeClaimSpec{
				StorageClassName: &standardClass,
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			}

			webserver.Spec.Volume.VolumeClaimTemplates = append(webserver.Spec.Volume.VolumeClaimTemplates, pvcSpec)

			createWebServer(webserver)

			By("Create a test file")
			podName := waitForPod()

			// Check that PVC exists
			pvcName := fmt.Sprintf("jws-img-0-%s", podName)

			pvc := &corev1.PersistentVolumeClaim{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: pvcName, Namespace: namespace}, pvc)
			}, time.Second*30, time.Second).Should(Succeed())

			storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(storage.String()).To(Equal("1Gi"))

			containerName := getPodContainerName(podName)
			mountPath := resolveMountPath(podName)
			filePath := fmt.Sprintf("%s/%s", mountPath, testFileName)
			writeDataToFile(podName, containerName, filePath, testContent)

			By("Verifying Write Access")
			verifyFileContent(podName, containerName, filePath, testContent)

			By("Force pod to restart and verify persistence after recreation")
			forcePodRestart(namespace, podName)
			newPodName := waitForPod()
			Expect(newPodName).To(Equal(podName))

			newContainerName := getPodContainerName(newPodName)
			verifyFileContent(newPodName, newContainerName, filePath, testContent)
		})
	})

	/*
	   Test: ReadWriteOncePod (RWOP)
	   Goal: Verify strict single-pod volume access mode using templates.
	*/
	It("ReadWriteOncePodTest", func() {
		testContent := "RWOP-Exclusive-Check"

		By("Creating WebServer with VolumeClaimTemplate (RWOP)")
		pvcSpec := corev1.PersistentVolumeClaimSpec{
			StorageClassName: &standardClass,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOncePod, // RWOP
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		}
		webserver.Spec.Volume.VolumeClaimTemplates = append(webserver.Spec.Volume.VolumeClaimTemplates, pvcSpec)
		createWebServer(webserver)

		By("Waiting for the main Pod")
		podName := waitForPod()
		containerName := getPodContainerName(podName)
		mountPath := resolveMountPath(podName)
		filePath := fmt.Sprintf("%s/%s", mountPath, testFileName)

		writeDataToFile(podName, containerName, filePath, testContent)

		// Checking exclusivity (RWOP)
		By("Verifying Access Exclusivity: Attempting to attach the same PVC to a second Pod on the SAME Node")

		mainPod := &corev1.Pod{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: namespace}, mainPod)).To(Succeed())

		nodeName := mainPod.Spec.NodeName
		var claimName string
		for _, vol := range mainPod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil {
				claimName = vol.PersistentVolumeClaim.ClaimName
				break
			}
		}
		Expect(claimName).ToNot(BeEmpty(), "Could not find PVC name in running pod")
		fmt.Printf("Main Pod is on node: %s, using PVC: %s\n", nodeName, claimName)

		// Creating a "Conflict" pod (Rogue Pod)
		roguePodName := "rogue-pod-rwop-test"
		roguePod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roguePodName,
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				// Put it on the same node. It would have worked for RWO, but not for RWOP.
				NodeName: nodeName,
				Containers: []corev1.Container{
					{
						Name:    "jws-rogue",
						Image:   testImg,
						Command: []string{"sleep", "3600"},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "vol",
								MountPath: "/data",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "vol",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: claimName, // Trying to pick up the same PVC
							},
						},
					},
				},
			},
		}

		Expect(k8sClient.Create(context.Background(), roguePod)).To(Succeed())

		By("Ensuring the second Pod fails to start due to RWOP lock")
		Consistently(func() bool {
			currentRogue := &corev1.Pod{}
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: roguePodName, Namespace: namespace}, currentRogue)
			if err != nil {
				return false
			}
			if currentRogue.Status.Phase == corev1.PodRunning {
				fmt.Printf("ERROR: Rogue pod managed to start! RWOP failed.\n")
				return true
			}
			return false
		}, time.Second*30, time.Second*2).Should(BeFalse(), "Rogue pod should NOT start because PVC is ReadWriteOncePod")

		fmt.Println("Success: Rogue pod was blocked.")

		// Deleting the rogue pod before restarting the main one
		By("Cleaning up Rogue Pod to release contention")
		Expect(k8sClient.Delete(context.Background(), roguePod)).To(Succeed())

		// We are waiting for the Rogue Pod to disappear so that the disk is definitely free for restart.
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: roguePodName, Namespace: namespace}, &corev1.Pod{})
			return apierrors.IsNotFound(err)
		}, time.Second*30, time.Second).Should(BeTrue(), "Rogue pod should be deleted before proceeding")

		By("Force pod to restart the Main Pod to verify persistence after recreation")
		forcePodRestart(namespace, podName)

		newPodName := waitForPod()
		Expect(newPodName).To(Equal(podName))

		newContainerName := getPodContainerName(newPodName)
		verifyFileContent(newPodName, newContainerName, filePath, testContent)
	})

	/*
		Test: Existing PersistentVolumeClaims List
		Goal: Verify attaching, detaching, and updating the list of pre-existing PVCs
		specified in .spec.volume.persistentVolumeClaims.
	*/
	It("PersistentVolumeClaimsListTest", func() {
		pvc1Name := "manual-pvc-1"
		pvc2Name := "manual-pvc-2"
		storageSize := "1Gi"

		// Function for creating a testing PVC
		createManualPVC := func(name string) {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &standardClass,
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(storageSize),
						},
					},
				},
			}
			createPVC(pvc)
		}

		// Check function: is there PVC in the feed volumes
		// Returns true if a PVC with the same name is mounted
		checkPodHasPVC := func(podName, targetPVCName string) bool {
			pod := &corev1.Pod{}
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: namespace}, pod)
			if err != nil {
				return false
			}
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == targetPVCName {
					return true
				}
			}
			return false
		}

		By("0. Pre-creating two manual PVCs")
		createManualPVC(pvc1Name)
		createManualPVC(pvc2Name)

		By("1. Creating WebServer with ONE PVC attached")
		webserver.Spec.Volume.PersistentVolumeClaims = []string{pvc1Name}
		createWebServer(webserver)

		podName := waitForPod()

		By("Verifying Pod has attached " + pvc1Name)
		Eventually(func() bool {
			return checkPodHasPVC(podName, pvc1Name)
		}, time.Second, time.Second*2).Should(BeTrue(), "Pod should have pvc1 attached")

		By("Verifying Pod does NOT have " + pvc2Name + " yet")
		Expect(checkPodHasPVC(podName, pvc2Name)).To(BeFalse())

		By("2. Updating WebServer to attach SECOND PVC")
		Eventually(func() error {
			currentWS := getWebServer(webserver.Name)
			currentWS.Spec.Volume.PersistentVolumeClaims = []string{pvc1Name, pvc2Name}
			return k8sClient.Update(context.Background(), currentWS)
		}, time.Minute, time.Second*2).Should(Succeed())

		By("Verifying Pod has BOTH PVCs attached")
		Eventually(func() bool {
			name := waitForPod()
			return checkPodHasPVC(name, pvc1Name) && checkPodHasPVC(name, pvc2Name)
		}, time.Minute*3, time.Second*2).Should(BeTrue(), "Pod should have both PVCs attached after update")

		By("3. Updating WebServer to REMOVE the second PVC")
		Eventually(func() error {
			currentWS := getWebServer(webserver.Name)
			currentWS.Spec.Volume.PersistentVolumeClaims = []string{pvc1Name}
			return k8sClient.Update(context.Background(), currentWS)
		}, time.Minute, time.Second*2).Should(Succeed())

		By("Verifying Pod has only PVC1 attached")
		Eventually(func() bool {
			name := waitForPod()
			hasPVC1 := checkPodHasPVC(name, pvc1Name)
			hasPVC2 := checkPodHasPVC(name, pvc2Name)

			return hasPVC1 && !hasPVC2
		}, time.Minute*3, time.Second*2).Should(BeTrue(), "Pod should maintain PVC1 but detach PVC2")

		By("4. Updating WebServer to DETACH ALL PVCs from list")
		Eventually(func() error {
			currentWS := getWebServer(webserver.Name)

			currentWS.Spec.Volume.PersistentVolumeClaims = []string{} // или nil
			return k8sClient.Update(context.Background(), currentWS)
		}, time.Minute, time.Second*2).Should(Succeed())

		By("Verifying Pod has NO custom PVCs attached")
		Eventually(func() bool {
			name := waitForPod()
			hasPVC1 := checkPodHasPVC(name, pvc1Name)
			hasPVC2 := checkPodHasPVC(name, pvc2Name)

			return !hasPVC1 && !hasPVC2
		}, time.Minute*3, time.Second*2).Should(BeTrue(), "Pod should have no custom PVCs attached")
	})

	It("SecretsMountTest", func() {
		secret1Name := "test-secret-1"
		secret2Name := "test-secret-2"

		key1 := "username"
		val1 := "admin_user"
		key2 := "password"
		val2 := "super_secure_pass"

		createManualSecret := func(name, key, value string) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				StringData: map[string]string{
					key: value,
				},
				Type: corev1.SecretTypeOpaque,
			}
			createSecret(secret)
		}

		verifySecretContent := func(podName, secretName, key, expectedValue string) {
			fullPath := fmt.Sprintf("/secrets/%s/%s", secretName, key)
			containerName := "jws-img"

			cmd := []string{"cat", fullPath}
			stdout, _, err := executeCommandOnPod(podName, containerName, cmd)

			Expect(err).NotTo(HaveOccurred(), "Failed to read secret file: "+fullPath)
			Expect(strings.TrimSpace(stdout)).To(Equal(expectedValue), "Secret content mismatch in "+fullPath)
		}

		verifySecretGone := func(podName, secretName, key string) {
			fullPath := fmt.Sprintf("/secrets/%s/%s", secretName, key)
			containerName := "jws-img"
			cmd := []string{"ls", fullPath}
			_, _, err := executeCommandOnPod(podName, containerName, cmd)
			Expect(err).To(HaveOccurred(), "Secret file should NOT exist: "+fullPath)
		}

		By("0. Pre-creating two manual Secrets")
		createManualSecret(secret1Name, key1, val1)
		createManualSecret(secret2Name, key2, val2)

		defer func() {
			deleteSecret(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret1Name, Namespace: namespace}})
			deleteSecret(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret2Name, Namespace: namespace}})
		}()

		By("1. Creating WebServer with SECRET 1 attached")
		webserver.Spec.Volume.Secrets = []string{secret1Name}
		createWebServer(webserver)

		podName := waitForPod()

		By("Verifying Secret 1 is mounted and readable")
		Eventually(func() error {
			fullPath := fmt.Sprintf("/secrets/%s/%s", secret1Name, key1)
			_, _, err := executeCommandOnPod(podName, "jws-img", []string{"cat", fullPath})
			return err
		}, time.Second*30, time.Second*2).Should(Succeed())

		verifySecretContent(podName, secret1Name, key1, val1)
		verifySecretGone(podName, secret2Name, key2)

		By("2. Updating WebServer to attach SECRET 2")
		Eventually(func() error {
			currentWS := getWebServer(webserver.Name)
			currentWS.Spec.Volume.Secrets = []string{secret1Name, secret2Name}
			return k8sClient.Update(context.Background(), currentWS)
		}, time.Second*30, time.Second*2).Should(Succeed())

		By("Verifying BOTH Secrets are mounted (waiting for pod rotation)")
		Eventually(func() bool {
			currentPodName := waitForPod()

			path1 := fmt.Sprintf("/secrets/%s/%s", secret1Name, key1)
			path2 := fmt.Sprintf("/secrets/%s/%s", secret2Name, key2)

			_, _, err1 := executeCommandOnPod(currentPodName, "jws-img", []string{"ls", path1})
			if err1 != nil {
				return false
			}

			_, _, err2 := executeCommandOnPod(currentPodName, "jws-img", []string{"ls", path2})
			if err2 != nil {
				return false
			}

			podName = currentPodName
			return true
		}, time.Second*30, time.Second*2).Should(BeTrue(), "Both secrets should eventually appear in the pod")

		verifySecretContent(podName, secret1Name, key1, val1)
		verifySecretContent(podName, secret2Name, key2, val2)

		By("3. Updating WebServer to REMOVE ALL secrets")
		Eventually(func() error {
			currentWS := getWebServer(webserver.Name)
			currentWS.Spec.Volume.Secrets = []string{}
			return k8sClient.Update(context.Background(), currentWS)
		}, time.Second*30, time.Second*2).Should(Succeed())

		By("Verifying Secrets are detached")
		Eventually(func() bool {
			currentPodName := waitForPod()

			path1 := fmt.Sprintf("/secrets/%s/%s", secret1Name, key1)
			_, _, err := executeCommandOnPod(currentPodName, "jws-img", []string{"ls", path1})
			return err != nil
		}, time.Second*30, time.Second*2).Should(BeTrue(), "Secret files should disappear")
	})

	/*
	   Test: ConfigMaps Mounting
	   Goal: Verify attaching, updating, and detaching ConfigMaps listed in .spec.volume.configMaps.
	   Mount Path Logic: Usually mounted at /configmaps/<cm-name>/<key>
	*/
	It("ConfigMapsMountTest", func() {
		cm1Name := "test-cm-1"
		cm2Name := "test-cm-2"

		key1 := "config.properties"
		val1 := "setting=true"
		key2 := "extra-config.xml"
		val2 := "<root>data</root>"

		// Helper to create ConfigMap
		createManualConfigMap := func(name, key, value string) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: map[string]string{
					key: value,
				},
			}
			Eventually(func() error {
				return k8sClient.Create(context.Background(), cm)
			}, "10s", "1s").Should(Succeed())
			fmt.Printf("ConfigMap %s created\n", name)
		}

		// Helper to verify content inside the pod
		verifyCMContent := func(podName, cmName, key, expectedValue string) {
			fullPath := fmt.Sprintf("/configmaps/%s/%s", cmName, key)

			// Use Eventually to handle potential transient errors like "container not found"
			Eventually(func() error {
				// Dynamically get the container name to be safe
				containerName := getPodContainerName(podName)
				cmd := []string{"cat", fullPath}
				stdout, _, err := executeCommandOnPod(podName, containerName, cmd)
				if err != nil {
					return err
				}
				if strings.TrimSpace(stdout) != expectedValue {
					return fmt.Errorf("content mismatch: expected '%s', got '%s'", expectedValue, strings.TrimSpace(stdout))
				}
				return nil
			}, "30s", "2s").Should(Succeed(), "Failed to verify ConfigMap content in "+fullPath)
		}

		deleteManualConfigMap := func(name string) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
			_ = k8sClient.Delete(context.Background(), cm)
		}

		By("0. Pre-creating two manual ConfigMaps")
		createManualConfigMap(cm1Name, key1, val1)
		createManualConfigMap(cm2Name, key2, val2)

		defer func() {
			deleteManualConfigMap(cm1Name)
			deleteManualConfigMap(cm2Name)
		}()

		By("1. Creating WebServer with ConfigMap 1 attached")
		webserver.Spec.Volume.ConfigMaps = []string{cm1Name}
		createWebServer(webserver)

		// Wait for the pod to be created and running
		podName := waitForPod()

		By("Verifying ConfigMap 1 is mounted and readable")
		// Check if file exists first
		Eventually(func() error {
			fullPath := fmt.Sprintf("/configmaps/%s/%s", cm1Name, key1)
			// Dynamically get container name
			containerName := getPodContainerName(podName)
			_, _, err := executeCommandOnPod(podName, containerName, []string{"cat", fullPath})
			return err
		}, time.Minute, time.Second*2).Should(Succeed(), "ConfigMap 1 file not accessible")

		verifyCMContent(podName, cm1Name, key1, val1)

		By("2. Updating WebServer to attach ConfigMap 2")
		Eventually(func() error {
			currentWS := getWebServer(webserver.Name)
			currentWS.Spec.Volume.ConfigMaps = []string{cm1Name, cm2Name}
			return k8sClient.Update(context.Background(), currentWS)
		}, time.Minute, time.Second*2).Should(Succeed())

		By("Verifying BOTH ConfigMaps are mounted (waiting for pod rotation)")
		Eventually(func() bool {
			// Get current pod name (it might be the same name but different UID/instance)
			currentPodName := waitForPod()

			// Ensure we can talk to the container
			containerName := ""
			pod := &corev1.Pod{}
			if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: currentPodName, Namespace: namespace}, pod); err == nil {
				if len(pod.Spec.Containers) > 0 {
					containerName = pod.Spec.Containers[0].Name
				}
			}
			if containerName == "" {
				return false
			}

			path1 := fmt.Sprintf("/configmaps/%s/%s", cm1Name, key1)
			path2 := fmt.Sprintf("/configmaps/%s/%s", cm2Name, key2)

			_, _, err1 := executeCommandOnPod(currentPodName, containerName, []string{"ls", path1})
			if err1 != nil {
				return false
			}

			_, _, err2 := executeCommandOnPod(currentPodName, containerName, []string{"ls", path2})
			if err2 != nil {
				return false
			}

			// Update podName variable for subsequent content checks
			podName = currentPodName
			return true
		}, time.Minute*3, time.Second*2).Should(BeTrue(), "Both ConfigMaps should eventually appear")

		verifyCMContent(podName, cm1Name, key1, val1)
		verifyCMContent(podName, cm2Name, key2, val2)

		By("3. Updating WebServer to REMOVE ALL ConfigMaps")
		Eventually(func() error {
			currentWS := getWebServer(webserver.Name)
			currentWS.Spec.Volume.ConfigMaps = []string{}
			return k8sClient.Update(context.Background(), currentWS)
		}, time.Minute, time.Second*2).Should(Succeed())

		By("Verifying ConfigMaps are detached")
		Eventually(func() bool {
			currentPodName := waitForPod()

			containerName := ""
			pod := &corev1.Pod{}
			if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: currentPodName, Namespace: namespace}, pod); err == nil {
				if len(pod.Spec.Containers) > 0 {
					containerName = pod.Spec.Containers[0].Name
				}
			}
			if containerName == "" {
				return false // Not ready yet
			}

			path1 := fmt.Sprintf("/configmaps/%s/%s", cm1Name, key1)

			_, _, err := executeCommandOnPod(currentPodName, containerName, []string{"ls", path1})
			// We expect an error here (file not found)
			return err != nil
		}, time.Minute*3, time.Second*2).Should(BeTrue(), "ConfigMap files should disappear")
	})
})
